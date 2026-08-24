package goystore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LocalBlobStore implements the BlobStore contract using the local filesystem.
type LocalBlobStore struct {
	basePath string
}

// NewLocalBlobStore creates a new LocalBlobStore with the specified base directory.
func NewLocalBlobStore(basePath string) *LocalBlobStore {
	if basePath == "" {
		basePath = "./data/blobs"
	}
	return &LocalBlobStore{basePath: basePath}
}

func (s *LocalBlobStore) resolvePath(key string) (string, error) {
	if strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid blob key: directory traversal detected")
	}

	cleanKey := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+key)), "/")
	if cleanKey == "" || strings.HasPrefix(cleanKey, "..") {
		return "", fmt.Errorf("invalid blob key: directory traversal detected")
	}

	target := filepath.Join(s.basePath, filepath.FromSlash(cleanKey))
	rel, err := filepath.Rel(s.basePath, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid blob key: directory traversal detected")
	}
	return target, nil
}

func (s *LocalBlobStore) metaPath(blobPath string) string {
	return blobPath + ".meta.json"
}

// Put writes a blob and its optional metadata to the filesystem.
func (s *LocalBlobStore) Put(ctx context.Context, key string, data []byte, metadata *Metadata) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	targetPath, err := s.resolvePath(key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create directories for blob: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write blob file: %w", err)
	}

	if metadata != nil {
		metaJSON, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal blob metadata: %w", err)
		}
		if err := os.WriteFile(s.metaPath(targetPath), metaJSON, 0644); err != nil {
			return fmt.Errorf("failed to write blob metadata: %w", err)
		}
	}

	return nil
}

// Get reads a blob and its metadata from the filesystem.
func (s *LocalBlobStore) Get(ctx context.Context, key string) ([]byte, *Metadata, bool, error) {
	select {
	case <-ctx.Done():
		return nil, nil, false, ctx.Err()
	default:
	}

	targetPath, err := s.resolvePath(key)
	if err != nil {
		return nil, nil, false, err
	}

	data, err := os.ReadFile(targetPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to read blob: %w", err)
	}

	var meta *Metadata
	metaData, err := os.ReadFile(s.metaPath(targetPath))
	if err == nil {
		var m Metadata
		if json.Unmarshal(metaData, &m) == nil {
			meta = &m
		}
	}

	return data, meta, true, nil
}

// Delete removes the blob and its metadata from the filesystem.
func (s *LocalBlobStore) Delete(ctx context.Context, key string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	targetPath, err := s.resolvePath(key)
	if err != nil {
		return err
	}

	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to delete blob: %w", err)
	}
	_ = os.Remove(s.metaPath(targetPath))

	return nil
}

// List lists all blobs matching an optional prefix.
func (s *LocalBlobStore) List(ctx context.Context, prefix *string) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if _, err := os.Stat(s.basePath); errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}

	var results []string
	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".meta.json") {
			return nil
		}

		rel, err := filepath.Rel(s.basePath, path)
		if err != nil {
			return err
		}

		relSlash := filepath.ToSlash(rel)
		if prefix == nil || strings.HasPrefix(relSlash, *prefix) {
			results = append(results, relSlash)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list blobs: %w", err)
	}

	sort.Strings(results)
	return results, nil
}

// PresignURL returns a local file URL for the blob.
func (s *LocalBlobStore) PresignURL(ctx context.Context, key string, _ time.Duration) (string, error) {
	targetPath, err := s.resolvePath(key)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(targetPath)
	if err != nil {
		abs = targetPath
	}
	return "file://" + filepath.ToSlash(abs), nil
}
