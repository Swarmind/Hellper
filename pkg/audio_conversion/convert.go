package audioconversion

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func OpusToWav(oggData []byte) ([]byte, error) {
	cmd := exec.Command("sox", "-t", "opus", "-", "-t", "wav", "-") // Use pipes

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		defer stdin.Close()
		_, err := stdin.Write(oggData)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to stdin: %v\n", err)
		}
	}()

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("sox command failed: %v\nStderr: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}
