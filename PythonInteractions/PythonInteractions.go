package PythonInteractions

import (
	"fmt"
	"os"
	"os/exec"
)

func StartPythonRepricer(updates int, isFeltrinelli bool) {
	// Start python repricer if there is any book to update.
	if updates <= 0 {
		return
	}
	var repricerString string
	if isFeltrinelli {
		repricerString = "--feltrinelliRepricer"
	} else {
		repricerString = "--repricer"
	}
	cmd := exec.Command("/bin/bash", "/home/mattia/repricer/start_repricer.sh", repricerString)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_, err := fmt.Fprintf(os.Stderr, "Error in starting repricer script from GoLang to Python: %s", err)
		if err != nil {
			return
		}
		return
	}
	fmt.Printf("%s\n", output)

}
