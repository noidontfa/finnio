package abr

import (
	"fmt"
	"io"
	"media/internal/ffmpeg"
	"os"
	"path/filepath"
	"shared/helper"
)

type HandlerV2 struct {
	ffm *ffmpeg.Runner
}

func NewHandlerV2(ffm *ffmpeg.Runner) *HandlerV2 {
	return &HandlerV2{ffm: ffm}
}

func (h *HandlerV2) Handle(request Request, outputFolder string) error {
	enabledVariants := enabledVariants(variants)
	if len(enabledVariants) == 0 {
		return fmt.Errorf("no enabled variants")
	}
	for _, v := range enabledVariants {
		err := os.MkdirAll(filepath.Join(outputFolder, v.Label), 0o755)
		if err != nil {
			return err
		}
	}

	err := h.ensureMaster(outputFolder, enabledVariants)
	if err != nil {
		return err
	}

	switch request.SourceType {
	case TS_FILE:
		return h.encodeTS(request.SegmentFile, outputFolder, enabledVariants)
	case INDEX_FILE:
		return h.encodeIndex(request.SegmentFile, outputFolder, enabledVariants)
	}
	return nil
}

func (h *HandlerV2) encodeTS(inputFile string, outputFolder string, enabledVariants []Variant) error {
	done, stop := helper.Done()
	defer stop()

	funcs := make([]func() error, 0, len(enabledVariants))
	for _, v := range enabledVariants {
		funcs = append(funcs, func(v Variant) func() error {
			return func() error {
				return h.encodeVariant(inputFile, outputFolder, v)
			}
		}(v))
	}

	funChns := helper.Parallel(done, funcs...)
	var firstErr error
	for _, funChn := range funChns {
		if err := <-funChn; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h *HandlerV2) encodeVariant(inputFile string, outputFolder string, v Variant) error {
	name := filepath.Base(inputFile)
	tmpPath := filepath.Join(outputFolder, v.Label, name+".tmp")
	path := filepath.Join(outputFolder, v.Label, name)

	err := h.ffm.Run(_encodeArgs(inputFile, tmpPath, v)...)
	if err != nil {
		return err
	}

	err = os.Rename(tmpPath, path)
	if err != nil {
		return err
	}

	return nil
}

func (h *HandlerV2) encodeIndex(inputFile string, outputFolder string, enabledVariants []Variant) error {
	done, stop := helper.Done()
	defer stop()

	funcs := make([]func() error, 0, len(enabledVariants))
	for _, v := range enabledVariants {
		funcs = append(funcs, func(v Variant) func() error {
			return func() error {
				return copyFile(inputFile, filepath.Join(outputFolder, v.Label, "index.m3u8"))
			}
		}(v))
	}

	funChns := helper.Parallel(done, funcs...)
	var firstErr error
	for _, funChn := range funChns {
		if err := <-funChn; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h *HandlerV2) ensureMaster(outputFolder string, enabledVariants []Variant) error {
	if err := os.MkdirAll(outputFolder, 0o755); err != nil {
		return err
	}

	masterFile := filepath.Join(outputFolder, "master.m3u8")
	if _, err := os.Stat(masterFile); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	content := masterPlaylist(enabledVariants)
	tmp := masterFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, masterFile)
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// Create/truncate destination file.
	// If it already exists, it will be overwritten.
	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	if err != nil {
		return err
	}

	return dest.Sync()
}

func _encodeArgs(inputFile, outputFile string, v Variant) []string {
	args := []string{
		"-hide_banner", "-y",
		"-copyts",
		"-i", inputFile,
		"-filter_complex", fmt.Sprintf("[0:v]scale=%d:%d[s%s]", v.Width, v.Height, v.Label),
		"-map", fmt.Sprintf("[s%s]", v.Label),
		"-f", "mpegts",
		"-muxdelay", "0", "-muxpreload", "0",
		outputFile,
	}
	return args
}
