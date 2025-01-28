package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type ffprobeReturn struct {
	Streams []struct {
		Width  int `json:"width,omitempty"`
		Height int `json:"height,omitempty"`
	} `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	output := bytes.Buffer{}
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	jsonReturn := ffprobeReturn{}

	if err := json.Unmarshal(output.Bytes(), &jsonReturn); err != nil {
		return "", err
	}

	if len(jsonReturn.Streams) == 0 {
		return "", fmt.Errorf("No streams found in video metadata")
	}

	if int(jsonReturn.Streams[0].Width)/int(jsonReturn.Streams[0].Height) == int(16/9) {
		return "16:9", nil
	}
	if int(jsonReturn.Streams[0].Width)/int(jsonReturn.Streams[0].Height) == int(9/16) {
		return "9:16", nil
	}
	return "other", nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)
	err := cmd.Run()
	if err != nil {
		return "", nil
	}

	return outputFilePath, nil
}
