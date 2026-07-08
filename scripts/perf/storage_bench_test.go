//go:build bench

package perf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/storage"
)

func newLocalBackend(b *testing.B) *storage.LocalBackend {
	b.Helper()
	root := b.TempDir()
	backend, err := storage.NewLocalBackend(storage.LocalConfig{
		RootPath: root,
		FileMode: "0644",
		DirMode:  "0755",
	})
	if err != nil {
		b.Fatalf("NewLocalBackend: %v", err)
	}
	b.Cleanup(func() {
		_ = backend.Close()
		_ = os.RemoveAll(root)
	})
	return backend
}

func BenchmarkStorage_UploadBytes_1KiB(b *testing.B) {
	benchmarkStorageUploadBytes(b, 1024)
}

func BenchmarkStorage_UploadBytes_1MiB(b *testing.B) {
	benchmarkStorageUploadBytes(b, 1024*1024)
}

func benchmarkStorageUploadBytes(b *testing.B, size int) {
	backend := newLocalBackend(b)
	ctx := context.Background()
	payload := bytes.Repeat([]byte("x"), size)
	b.SetBytes(int64(size))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		path := filepath.ToSlash(filepath.Join("bench", fmt.Sprintf("up-%d.bin", i)))
		if err := backend.UploadBytes(ctx, path, payload, "application/octet-stream"); err != nil {
			b.Fatalf("UploadBytes: %v", err)
		}
	}
}

func BenchmarkStorage_Download_1KiB(b *testing.B) {
	benchmarkStorageDownload(b, 1024)
}

func BenchmarkStorage_Download_1MiB(b *testing.B) {
	benchmarkStorageDownload(b, 1024*1024)
}

func benchmarkStorageDownload(b *testing.B, size int) {
	backend := newLocalBackend(b)
	ctx := context.Background()
	payload := bytes.Repeat([]byte("y"), size)
	const path = "bench/download.bin"
	if err := backend.UploadBytes(ctx, path, payload, "application/octet-stream"); err != nil {
		b.Fatalf("seed UploadBytes: %v", err)
	}

	b.SetBytes(int64(size))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rc, err := backend.Download(ctx, path)
		if err != nil {
			b.Fatalf("Download: %v", err)
		}
		n, err := io.Copy(io.Discard, rc)
		_ = rc.Close()
		if err != nil {
			b.Fatalf("read body: %v", err)
		}
		if n != int64(size) {
			b.Fatalf("got %d bytes, want %d", n, size)
		}
	}
}

func BenchmarkStorage_Exists(b *testing.B) {
	backend := newLocalBackend(b)
	ctx := context.Background()
	const path = "bench/exists.bin"
	if err := backend.UploadBytes(ctx, path, []byte("exists"), "text/plain"); err != nil {
		b.Fatalf("seed UploadBytes: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ok, err := backend.Exists(ctx, path)
		if err != nil {
			b.Fatalf("Exists: %v", err)
		}
		if !ok {
			b.Fatal("expected file to exist")
		}
	}
}

func BenchmarkStorage_UploadDownloadRoundTrip(b *testing.B) {
	backend := newLocalBackend(b)
	ctx := context.Background()
	const size = 64 * 1024
	payload := bytes.Repeat([]byte("z"), size)
	b.SetBytes(int64(size))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		path := filepath.ToSlash(filepath.Join("bench", "rt", fmt.Sprintf("%d.bin", i)))
		if err := backend.UploadBytes(ctx, path, payload, "application/octet-stream"); err != nil {
			b.Fatalf("UploadBytes: %v", err)
		}
		rc, err := backend.Download(ctx, path)
		if err != nil {
			b.Fatalf("Download: %v", err)
		}
		_, err = io.Copy(io.Discard, rc)
		_ = rc.Close()
		if err != nil {
			b.Fatalf("read body: %v", err)
		}
		if err := backend.Delete(ctx, path); err != nil {
			b.Fatalf("Delete: %v", err)
		}
	}
}
