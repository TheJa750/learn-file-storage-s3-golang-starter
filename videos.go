package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os/exec"
)

func getVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)

	var buff bytes.Buffer
	cmd.Stdout = &buff

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	var vidSize VideoInfo
	if err = json.Unmarshal(buff.Bytes(), &vidSize); err != nil {
		return "", err
	}

	width := float64(vidSize.Streams[0].Width)
	height := float64(vidSize.Streams[0].Height)

	ratio := width / height
	landscape := float64(16) / float64(9)
	portrait := float64(9) / float64(16)

	if math.Abs(landscape-ratio) < 0.1 {
		return "landscape", nil
	} else if math.Abs(portrait-ratio) < 0.1 {
		return "portrait", nil
	} else {
		return "other", nil
	}
}

func processVideoForFastStart(filepath string) (string, error) {
	outputPath := filepath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath)

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return outputPath, nil
}
