package main

// createuvm does what it says on the tin. Simple test utility for looking
// at startup timing.

import (
	"bufio"
	"fmt"
	"os"

	"github.com/Microsoft/hcsshim/internal/uvm"
)

func main() {

	fmt.Println("Creating...")
	lcowUVM, err := uvm.Create(&uvm.UVMOptions{OperatingSystem: "linux", ID: "uvm"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create utility VM: %s", err)
		os.Exit(-1)
	}

	fmt.Print("Created. Press 'Enter' to start...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')

	fmt.Println("Starting...")
	if err := lcowUVM.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start utility VM: %s", err)
		os.Exit(-1)
	}

	fmt.Print("Started. Use `hcsdiag console -uvm uvm`. Press 'Enter' to terminate...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')

	lcowUVM.Terminate()
	os.Exit(0)
}
