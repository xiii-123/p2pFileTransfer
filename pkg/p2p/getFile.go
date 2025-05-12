package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"p2pFileTransfer/pkg/file"
	"sync"
)

const (
	ConcurrentLimit = 16
)

func (p *P2PService) loadMetaData(ctx context.Context, fileHash string) (*file.MetaData, error) {
	metaInfo, err := p.Get(ctx, fileHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	var metaData file.MetaData
	if err := json.Unmarshal([]byte(metaInfo), &metaData); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	logrus.Infof("Metadata parsed: FileName=%s, FileSize=%d, Leaves=%d chunks",
		metaData.FileName, metaData.FileSize, len(metaData.Leaves))

	return &metaData, nil
}

func (p *P2PService) downloadChunksConcurrently(
	ctx context.Context,
	leaves []file.ChunkData,
	concurrency int,
	handleChunk func(i int, chunk file.ChunkData, offset int64, data []byte) error,
) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(leaves))
	sem := make(chan struct{}, concurrency)

	var offset int64 = 0
	for i, chunk := range leaves {
		i, chunk := i, chunk
		chunkOffset := offset
		offset += int64(chunk.ChunkSize)

		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 下载 chunk
			peers, err := p.DHT.GetClosestPeers(ctx, string(chunk.ChunkHash))
			if err != nil {
				errCh <- fmt.Errorf("chunk %d: get peers failed: %w", i, err)
				return
			}
			bestPeer, err := p.SelectAvailablePeer(ctx, peers, string(chunk.ChunkHash))
			if err != nil {
				errCh <- fmt.Errorf("chunk %d: select peer failed: %w", i, err)
				return
			}
			chunkData, err := p.DownloadChunk(ctx, bestPeer, string(chunk.ChunkHash))
			if err != nil {
				errCh <- fmt.Errorf("chunk %d: get chunk failed: %w", i, err)
				return
			}
			if err := handleChunk(i, chunk, chunkOffset, chunkData); err != nil {
				errCh <- err
				return
			}
		}()
	}

	wg.Wait()
	close(errCh)
	if len(errCh) > 0 {
		return <-errCh
	}
	return nil
}

func (p *P2PService) GetFileOrdered(ctx context.Context, fileHash string, f io.ReadWriter) error {
	metaData, err := p.loadMetaData(ctx, fileHash)
	if err != nil {
		return err
	}

	results := make([][]byte, len(metaData.Leaves))
	err = p.downloadChunksConcurrently(ctx, metaData.Leaves, ConcurrentLimit, func(i int, chunk file.ChunkData, _ int64, data []byte) error {
		results[i] = data
		logrus.Infof("Chunk %d downloaded", i)
		return nil
	})
	if err != nil {
		return err
	}

	for i, data := range results {
		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("write chunk %d failed: %w", i, err)
		}
	}
	logrus.Infof("File %s downloaded successfully (ordered)", metaData.FileName)
	return nil
}

func (p *P2PService) GetFileRandom(ctx context.Context, fileHash string, f io.ReadWriter) error {
	writeAtFile, ok := f.(io.WriterAt)
	if !ok {
		return fmt.Errorf("target writer does not support WriterAt")
	}

	metaData, err := p.loadMetaData(ctx, fileHash)
	if err != nil {
		return err
	}

	err = p.downloadChunksConcurrently(ctx, metaData.Leaves, ConcurrentLimit, func(i int, chunk file.ChunkData, offset int64, data []byte) error {
		if _, err := writeAtFile.WriteAt(data, offset); err != nil {
			return fmt.Errorf("chunk %d: write failed at offset %d: %w", i, offset, err)
		}
		logrus.Infof("Chunk %d written at offset %d", i, offset)
		return nil
	})
	if err != nil {
		return err
	}

	logrus.Infof("File %s downloaded successfully (random)", metaData.FileName)
	return nil
}
