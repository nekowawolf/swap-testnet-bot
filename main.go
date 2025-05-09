package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	
	"github.com/nekowawolf/swap-testnet-bot/swap"
)

func main() {
	fmt.Println("\nSelect DEX:")
	fmt.Println("1. Apebond")
	fmt.Print("\nEnter your choice: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		swap.Apebond()
	default:
		fmt.Println("Invalid choice. Please select a valid option.")
		os.Exit(1)
	}
}