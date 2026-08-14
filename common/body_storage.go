package common

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// BodyStorage 请求体存储接口
type BodyStorage interface {
	io.ReadSeeker
	io.Closer
	// Bytes 获取全部内容
	Bytes() ([]byte, error)
	// Size 获取数据大小
	Size() int64
	// IsDisk 是否是磁盘存储
	IsDisk() bool
}

// ErrStorageClosed 存储已关闭错误
var ErrStorageClosed = fmt.Errorf("body storage is closed")

// memoryStorage 内存存储实现
type memoryStorage struct {
	data   []byte
	reader *bytes.Reader
	size   int64
	closed int32
	mu     sync.Mutex
}

func newMemoryStorage(data []byte) *memoryStorage {
	size := int64(len(data))
	IncrementMemoryBuffers(size)
	return &memoryStorage{
		data:   data,
		reader: bytes.NewReader(data),
		size:   size,
	}
}

func (m *memoryStorage) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return m.reader.Read(p)
}

func (m *memoryStorage) Seek(offset int64, whence int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return m.reader.Seek(offset, whence)
}

func (m *memoryStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.CompareAndSwapInt32(&m.closed, 0, 1) {
		DecrementMemoryBuffers(m.size)
	}
	return nil
}

func (m *memoryStorage) Bytes() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return nil, ErrStorageClosed
	}
	return m.data, nil
}

func (m *memoryStorage) Size() int64 {
	return m.size
}

func (m *memoryStorage) IsDisk() bool {
	return false
}

// diskStorage 磁盘存储实现
type diskStorage struct {
	file     *os.File
	filePath string
	size     int64
	closed   int32
	mu       sync.Mutex
}

func newDiskStorage(data []byte, cachePath string) (*diskStorage, error) {
	if !ReserveDiskCache(int64(len(data))) {
		return nil, fmt.Errorf("disk cache capacity exceeded")
	}
	defer ReleaseDiskCacheReservation(int64(len(data)))
	// 使用统一的缓存目录管理
	filePath, file, err := CreateDiskCacheFile(DiskCacheTypeBody)
	if err != nil {
		return nil, err
	}

	// 写入数据
	n, err := file.Write(data)
	if err != nil || n != len(data) {
		file.Close()
		os.Remove(filePath)
		if err == nil {
			err = io.ErrShortWrite
		}
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	// 重置文件指针
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	size := int64(n)
	IncrementDiskFiles(size)

	return &diskStorage{
		file:     file,
		filePath: filePath,
		size:     size,
	}, nil
}

func newDiskStorageFromReader(reader io.Reader, maxBytes int64, cachePath string) (*diskStorage, error) {
	if maxBytes <= 0 {
		return nil, ErrRequestBodyTooLarge
	}
	// 使用统一的缓存目录管理
	filePath, file, err := CreateDiskCacheFile(DiskCacheTypeBody)
	if err != nil {
		return nil, err
	}

	// Count the active file immediately, then account for written bytes in
	// chunks. Reserving maxBytes up front would reject valid small unknown-length
	// bodies whenever the body limit is larger than the remaining disk budget.
	IncrementDiskFiles(0)
	written := int64(0)
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(filePath)
		DecrementDiskFiles(written)
	}
	buffer := make([]byte, 64*1024)
	for {
		readSize := int64(len(buffer))
		if remaining := maxBytes - written + 1; remaining < readSize {
			readSize = remaining
		}
		if readSize <= 0 {
			cleanup()
			return nil, ErrRequestBodyTooLarge
		}
		n, readErr := reader.Read(buffer[:readSize])
		if n > 0 {
			n64 := int64(n)
			if !ReserveDiskCache(n64) {
				cleanup()
				return nil, fmt.Errorf("disk cache capacity exceeded")
			}
			writtenNow, writeErr := file.Write(buffer[:n])
			if writtenNow > 0 {
				CommitDiskCacheReservation(int64(writtenNow))
				written += int64(writtenNow)
			}
			if writtenNow < n {
				ReleaseDiskCacheReservation(int64(n - writtenNow))
				cleanup()
				if writeErr != nil {
					return nil, fmt.Errorf("failed to write to temp file: %w", writeErr)
				}
				return nil, io.ErrShortWrite
			}
			if writeErr != nil {
				cleanup()
				return nil, fmt.Errorf("failed to write to temp file: %w", writeErr)
			}
			if written > maxBytes {
				cleanup()
				return nil, ErrRequestBodyTooLarge
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			cleanup()
			return nil, fmt.Errorf("failed to read request body: %w", readErr)
		}
	}

	// 重置文件指针
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	return &diskStorage{
		file:     file,
		filePath: filePath,
		size:     written,
	}, nil
}

func (d *diskStorage) Read(p []byte) (n int, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if atomic.LoadInt32(&d.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return d.file.Read(p)
}

func (d *diskStorage) Seek(offset int64, whence int) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if atomic.LoadInt32(&d.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return d.file.Seek(offset, whence)
}

func (d *diskStorage) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if atomic.CompareAndSwapInt32(&d.closed, 0, 1) {
		d.file.Close()
		os.Remove(d.filePath)
		DecrementDiskFiles(d.size)
	}
	return nil
}

func (d *diskStorage) Bytes() ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if atomic.LoadInt32(&d.closed) == 1 {
		return nil, ErrStorageClosed
	}

	// 保存当前位置
	currentPos, err := d.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	// 移动到开头
	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// 读取全部内容
	data := make([]byte, d.size)
	_, err = io.ReadFull(d.file, data)
	if err != nil {
		return nil, err
	}

	// 恢复位置
	if _, err := d.file.Seek(currentPos, io.SeekStart); err != nil {
		return nil, err
	}

	return data, nil
}

func (d *diskStorage) Size() int64 {
	return d.size
}

func (d *diskStorage) IsDisk() bool {
	return true
}

// CreateBodyStorage 根据数据大小创建合适的存储
func CreateBodyStorage(data []byte) (BodyStorage, error) {
	size := int64(len(data))
	threshold := GetDiskCacheThresholdBytes()

	// 检查是否应该使用磁盘缓存
	if IsDiskCacheEnabled() && size >= threshold {
		storage, err := newDiskStorage(data, GetDiskCachePath())
		if err != nil {
			// Do not fall back to another large in-memory copy when disk cache is
			// enabled but full or unavailable.
			return nil, fmt.Errorf("failed to create disk storage: %w", err)
		}
		return storage, nil
	}

	return newMemoryStorage(data), nil
}

// CreateBodyStorageFromReader 从 Reader 创建存储（用于大请求的流式处理）
func CreateBodyStorageFromReader(reader io.Reader, contentLength int64, maxBytes int64) (BodyStorage, error) {
	threshold := GetDiskCacheThresholdBytes()

	// Unknown-length bodies must also be streamed to disk when disk caching is
	// enabled. Reading them with io.ReadAll defeats the purpose of the cache and
	// creates a request-sized memory spike for every concurrent upload.
	if IsDiskCacheEnabled() &&
		(contentLength <= 0 || contentLength >= threshold) {
		storage, err := newDiskStorageFromReader(reader, maxBytes, GetDiskCachePath())
		if err != nil {
			if IsRequestBodyTooLargeError(err) {
				return nil, err
			}
			// 磁盘存储失败，reader 已被消费，无法安全回退
			// 直接返回错误而非尝试回退（因为 reader 数据已丢失）
			return nil, fmt.Errorf("disk storage creation failed: %w", err)
		}
		IncrementDiskCacheHits()
		return storage, nil
	}

	// 使用内存读取
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrRequestBodyTooLarge
	}

	storage, err := CreateBodyStorage(data)
	if err != nil {
		return nil, err
	}
	// 如果最终使用内存存储，记录内存缓存命中
	if !storage.IsDisk() {
		IncrementMemoryCacheHits()
	} else {
		IncrementDiskCacheHits()
	}
	return storage, nil
}

// ReaderOnly wraps an io.Reader to hide io.Closer, preventing http.NewRequest
// from type-asserting io.ReadCloser and closing the underlying BodyStorage.
func ReaderOnly(r io.Reader) io.Reader {
	return struct{ io.Reader }{r}
}

// ReadBodyPrefix reads at most maxBytes without turning a disk-backed body
// into a full in-memory copy. The storage cursor is restored before returning.
func ReadBodyPrefix(storage BodyStorage, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(storage, int64(maxBytes)))
	_, seekErr := storage.Seek(0, io.SeekStart)
	if readErr != nil {
		return nil, readErr
	}
	if seekErr != nil {
		return nil, seekErr
	}
	return data, nil
}

// LimitCaptureBytes returns a bounded copy suitable for diagnostics. The copy
// avoids retaining a large request/response backing array in the log record.
func LimitCaptureBytes(data []byte, maxBytes int) []byte {
	if maxBytes <= 0 || len(data) == 0 {
		return nil
	}
	if len(data) <= maxBytes {
		return data
	}
	limited := make([]byte, maxBytes)
	copy(limited, data[:maxBytes])
	return limited
}

// CleanupOldCacheFiles 清理旧的缓存文件（用于启动时清理残留）
func CleanupOldCacheFiles() {
	// 使用统一的缓存管理
	CleanupOldDiskCacheFiles(5 * time.Minute)
}
