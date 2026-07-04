package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// SaveScreenshot copies the source image into
// dataDir/screenshots/{session}_{trade}_{timestamp}.{ext} and returns the path
// relative to the data dir (the form stored in trades.screenshot_path). The
// store owns this because it owns the data directory and naming convention.
func (s *Store) SaveScreenshot(sessionID, tradeID int64, srcPath string) (string, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("opening screenshot %s: %w", srcPath, err)
	}
	defer src.Close()

	ext := filepath.Ext(srcPath)
	name := fmt.Sprintf("%d_%d_%s%s", sessionID, tradeID, time.Now().Format("20060102T150405"), ext)
	relPath := filepath.Join(ScreenshotsDir, name)
	absPath := filepath.Join(s.dataDir, relPath)

	dst, err := os.Create(absPath)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", absPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("copying screenshot: %w", err)
	}
	return relPath, nil
}
