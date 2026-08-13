package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
)

type Runner struct {
	ffmPath     string
	ffprobePath string
}

func NewRunner() *Runner {
	return &Runner{ffmPath: "ffmpeg", ffprobePath: "ffprobe"}
}

func (r *Runner) Run(args ...string) error {
	cmd := exec.Command(r.ffmPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\n%s", err, output)
	}

	fmt.Println("--------------------------------")
	fmt.Println(string(output))
	fmt.Println("--------------------------------")

	return nil
}

func (r *Runner) RunContext(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, r.ffmPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\n%s", err, output)
	}

	// fmt.Println("--------------------------------")
	// fmt.Println(string(output))
	// fmt.Println("--------------------------------")

	return nil
}

func (r *Runner) RunFFProbe(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, r.ffprobePath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w\n%s", err, output)
	}

	return nil
}

func (r *Runner) RunFFProbeContext(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.ffprobePath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w\n%s", err, output)
	}

	return output, nil
}
