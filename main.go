package main

import (
	"bufio"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/nekowawolf/swap-testnet-bot/swap"
)

func main() {
	fmt.Println("Select DEX:")
	fmt.Println("1. Apebond")
	fmt.Print("Enter your choice: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice != "1" {
		fmt.Println("Invalid choice. Currently only Apebond is supported.")
		os.Exit(1)
	}

	fmt.Println("\nSwap direction:")
	fmt.Println("1. MON to WMON")
	fmt.Println("2. WMON to MON")
	fmt.Print("Enter your choice: ")
	directionChoice, _ := reader.ReadString('\n')
	directionChoice = strings.TrimSpace(directionChoice)

	var direction string
	switch directionChoice {
	case "1":
		direction = "MON_to_WMON"
	case "2":
		direction = "WMON_to_MON"
	default:
		fmt.Println("Invalid choice. Please select 1 or 2.")
		os.Exit(1)
	}

	fmt.Println("\nLoading wallet balances...")
	swap.ShowInitialBalances()

	fmt.Print("\nEnter amount to swap (in MON/WMON): ")
	amountInput, _ := reader.ReadString('\n')
	amountInput = strings.TrimSpace(amountInput)

	amount, err := strconv.ParseFloat(amountInput, 64)
	if err != nil || amount <= 0 {
		fmt.Println("Invalid amount. Please enter a positive number.")
		os.Exit(1)
	}

	amountWei := new(big.Float).Mul(big.NewFloat(amount), big.NewFloat(1e18))
	amountInt := new(big.Int)
	amountWei.Int(amountInt)

	fmt.Print("\nEnter number of swap to perform: ")
	numInput, _ := reader.ReadString('\n')
	numInput = strings.TrimSpace(numInput)

	numswap, err := strconv.Atoi(numInput)
	if err != nil || numswap < 1 {
		fmt.Println("Invalid number. Please enter a positive integer.")
		os.Exit(1)
	}

	fmt.Printf("\nPreparing to perform %d swap of %.4f %s to %s\n", 
		numswap, amount, strings.Split(direction, "_")[0], strings.Split(direction, "_")[2])
	fmt.Println("Starting swap...\n")

	swap.ApebondSwap(amountInt, numswap, direction)
}