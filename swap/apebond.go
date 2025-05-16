package swap

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
)

const (
	WMON_CONTRACT_ADDRESS    = "0x760AfE86e5de5fa0Ee542fc7B7B713e1c5425701"
	RPC_URL_MONAD            = "https://testnet-rpc.monad.xyz"
	CHAIN_ID_MONAD           = 10143
	GAS_PRICE_BUFFER_PERCENT = 0
	GAS_LIMIT_BUFFER_PERCENT = 10
	EXPLORER_BASE_MONAD      = "https://testnet.monadexplorer.com/tx/"
	DELAY_SECONDS_MONAD      = 2
)

var (
	cyan    = color.New(color.FgCyan).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	green   = color.New(color.FgGreen).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	blue    = color.New(color.FgBlue).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
)

type SwapResult struct {
	Success     bool
	WalletIndex int
	Cycle       int
	Direction   string
	TxHash      string
	Fee         string
	Amount      string
	Error       error
}

func loadWMonABI() (abi.ABI, error) {
	abiJSON := `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"src","type":"address"},{"indexed":true,"internalType":"address","name":"guy","type":"address"},{"indexed":false,"internalType":"uint256","name":"wad","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"dst","type":"address"},{"indexed":false,"internalType":"uint256","name":"wad","type":"uint256"}],"name":"Deposit","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"src","type":"address"},{"indexed":true,"internalType":"address","name":"dst","type":"address"},{"indexed":false,"internalType":"uint256","name":"wad","type":"uint256"}],"name":"Transfer","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"src","type":"address"},{"indexed":false,"internalType":"uint256","name":"wad","type":"uint256"}],"name":"Withdrawal","type":"event"},{"stateMutability":"payable","type":"fallback"},{"inputs":[{"internalType":"address","name":"","type":"address"},{"internalType":"address","name":"","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"guy","type":"address"},{"internalType":"uint256","name":"wad","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"decimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"deposit","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"dst","type":"address"},{"internalType":"uint256","name":"wad","type":"uint256"}],"name":"transfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"src","type":"address"},{"internalType":"address","name":"dst","type":"address"},{"internalType":"uint256","name":"wad","type":"uint256"}],"name":"transferFrom","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"wad","type":"uint256"}],"name":"withdraw","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
	return abi.JSON(strings.NewReader(abiJSON))
}

func Apebond() {
	reader := bufio.NewReader(os.Stdin)

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
	ShowInitialBalances()

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

	ApebondSwap(amountInt, numswap, direction)
}

func estimateGasLimit(client *ethclient.Client, from common.Address, to common.Address, value *big.Int, data []byte) (uint64, error) {
	msg := ethereum.CallMsg{
		From:  from,
		To:    &to,
		Value: value,
		Data:  data,
	}
	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		return 0, fmt.Errorf("failed to estimate gas: %v", err)
	}

	gasLimitWithBuffer := gasLimit * (100 + GAS_LIMIT_BUFFER_PERCENT) / 100
	return gasLimitWithBuffer, nil
}

func ApebondSwap(amount *big.Int, numswap int, direction string) {
	godotenv.Load()

	wallets := make([]string, 20)
	for i := 0; i < 20; i++ {
		wallets[i] = os.Getenv(fmt.Sprintf("PRIVATE_KEYS_WALLET%d", i+1))
	}

	var activeWallets []string
	for i, key := range wallets {
		if key != "" {
			activeWallets = append(activeWallets, key)
			log.Printf("Loaded Wallet #%d", i+1)
		}
	}

	if len(activeWallets) == 0 {
		log.Fatal("No valid private keys found in environment variables")
	}

	wmonABI, err := loadWMonABI()
	if err != nil {
		log.Fatalf("ABI error: %v", err)
	}

	results := make(chan SwapResult, numswap)
	var wg sync.WaitGroup
	walletMutexes := make([]sync.Mutex, len(activeWallets))

	amountFloat := new(big.Float).Quo(new(big.Float).SetInt(amount), big.NewFloat(1e18))
	tokenName := "WMON"
	if direction == "MON_to_WMON" {
		tokenName = "MON"
	}

	fmt.Printf("\nPreparing to perform %d swaps of %.4f %s to %s\n",
		numswap, amountFloat, tokenName, strings.Split(direction, "_")[2])
	fmt.Println("Starting swaps...\n")

	for i := 0; i < numswap; i++ {
        wg.Add(1)
        walletIndex := i % len(activeWallets)

        currentDirection := direction

        go func(swapNum int, walletIdx int, dir string) {
            defer wg.Done()
            time.Sleep(time.Duration(swapNum*DELAY_SECONDS_MONAD) * time.Second)
            walletMutexes[walletIdx].Lock()
            defer walletMutexes[walletIdx].Unlock()

            var result SwapResult
            if dir == "MON_to_WMON" {
                result = swapMONtoWMON(activeWallets[walletIdx], walletIdx+1, swapNum+1, amount, wmonABI) 
            } else {
                result = swapWMONtoMON(activeWallets[walletIdx], walletIdx+1, swapNum+1, amount, wmonABI) 
            }
            results <- result
        }(i, walletIndex, currentDirection)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    processSwapResults(results, numswap, wmonABI)
}

func getMONBalance(client *ethclient.Client, address common.Address) (*big.Int, error) {
	balance, err := client.BalanceAt(context.Background(), address, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get MON balance: %v", err)
	}
	return balance, nil
}

func getWMONBalance(client *ethclient.Client, address common.Address, wmonABI abi.ABI) (*big.Int, error) {
	wmonContract := common.HexToAddress(WMON_CONTRACT_ADDRESS)
	contract := bind.NewBoundContract(wmonContract, wmonABI, client, client, client)

	var result []interface{}
	err := contract.Call(nil, &result, "balanceOf", address)
	if err != nil {
		return nil, fmt.Errorf("failed to get WMON balance: %v", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no balance returned")
	}

	balance, ok := result[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid balance type")
	}

	return balance, nil
}

func swapMONtoWMON(privateKey string, walletIndex int, cycle int, amount *big.Int, wmonABI abi.ABI) SwapResult {
	client, err := ethclient.Dial(RPC_URL_MONAD)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "MON_to_WMON",
			Error:       fmt.Errorf("RPC connection failed: %v", err),
		}
	}
	defer client.Close()

	suggestedGasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "MON_to_WMON",
			Error:       fmt.Errorf("failed to get gas price: %v", err),
		}
	}
	bufferGasPrice := new(big.Int).Mul(suggestedGasPrice, big.NewInt(100+GAS_PRICE_BUFFER_PERCENT))
	bufferGasPrice.Div(bufferGasPrice, big.NewInt(100))

	pk, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "MON_to_WMON",
			Error:       fmt.Errorf("invalid private key: %v", err),
		}
	}

	fromAddress := crypto.PubkeyToAddress(pk.PublicKey)
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "MON_to_WMON",
			Error:       fmt.Errorf("failed to get nonce: %v", err),
		}
	}

	data, err := wmonABI.Pack("deposit")
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "MON_to_WMON",
			Error:       fmt.Errorf("failed to pack deposit data: %v", err),
		}
	}

	gasLimit, err := estimateGasLimit(client, fromAddress, common.HexToAddress(WMON_CONTRACT_ADDRESS), amount, data)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "MON_to_WMON",
			Error:       fmt.Errorf("gas estimation failed: %v", err),
		}
	}

	tx := types.NewTransaction(
		nonce,
		common.HexToAddress(WMON_CONTRACT_ADDRESS),
		amount,
		gasLimit,
		bufferGasPrice,
		data,
	)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(CHAIN_ID_MONAD)), pk)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "MON_to_WMON",
			Error:       fmt.Errorf("failed to sign transaction: %v", err),
		}
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "MON_to_WMON",
			Error:       fmt.Errorf("failed to send transaction: %v", err),
		}
	}

	receipt, err := bind.WaitMined(context.Background(), client, signedTx)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "MON_to_WMON",
			Error:       fmt.Errorf("transaction mining failed: %v", err),
		}
	}

	fee := new(big.Float).Quo(
		new(big.Float).SetInt(new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), bufferGasPrice)),
		new(big.Float).SetInt(big.NewInt(1e18)),
	)
	feeStr, _ := fee.Float64()

	amountFloat := new(big.Float).Quo(
		new(big.Float).SetInt(amount),
		big.NewFloat(1e18),
	)

	return SwapResult{
        Success:     true,
        WalletIndex: walletIndex,
        Cycle:       cycle,
        Direction:   "MON_to_WMON",
        TxHash:      signedTx.Hash().Hex(),
        Fee:         fmt.Sprintf("%.6f MON", feeStr),
        Amount:      fmt.Sprintf("%.4f MON", amountFloat),
    }
}

func swapWMONtoMON(privateKey string, walletIndex int, cycle int, amount *big.Int, wmonABI abi.ABI) SwapResult {
	client, err := ethclient.Dial(RPC_URL_MONAD)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "WMON_to_MON",
			Error:       fmt.Errorf("RPC connection failed: %v", err),
		}
	}
	defer client.Close()

	suggestedGasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "WMON_to_MON",
			Error:       fmt.Errorf("failed to get gas price: %v", err),
		}
	}
	bufferGasPrice := new(big.Int).Mul(suggestedGasPrice, big.NewInt(100+GAS_PRICE_BUFFER_PERCENT))
	bufferGasPrice.Div(bufferGasPrice, big.NewInt(100))

	pk, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "WMON_to_MON",
			Error:       fmt.Errorf("invalid private key: %v", err),
		}
	}

	fromAddress := crypto.PubkeyToAddress(pk.PublicKey)
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "WMON_to_MON",
			Error:       fmt.Errorf("failed to get nonce: %v", err),
		}
	}

	data, err := wmonABI.Pack("withdraw", amount)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "WMON_to_MON",
			Error:       fmt.Errorf("failed to pack withdraw data: %v", err),
		}
	}

	gasLimit, err := estimateGasLimit(client, fromAddress, common.HexToAddress(WMON_CONTRACT_ADDRESS), big.NewInt(0), data)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "WMON_to_MON",
			Error:       fmt.Errorf("gas estimation failed: %v", err),
		}
	}

	tx := types.NewTransaction(
		nonce,
		common.HexToAddress(WMON_CONTRACT_ADDRESS),
		big.NewInt(0),
		gasLimit,
		bufferGasPrice,
		data,
	)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(CHAIN_ID_MONAD)), pk)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "WMON_to_MON",
			Error:       fmt.Errorf("failed to sign transaction: %v", err),
		}
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "WMON_to_MON",
			Error:       fmt.Errorf("failed to send transaction: %v", err),
		}
	}

	receipt, err := bind.WaitMined(context.Background(), client, signedTx)
	if err != nil {
		return SwapResult{
			Success:     false,
			WalletIndex: walletIndex,
			Direction:   "WMON_to_MON",
			Error:       fmt.Errorf("transaction mining failed: %v", err),
		}
	}

	fee := new(big.Float).Quo(
		new(big.Float).SetInt(new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), bufferGasPrice)),
		new(big.Float).SetInt(big.NewInt(1e18)),
	)
	feeStr, _ := fee.Float64()

	amountFloat := new(big.Float).Quo(
		new(big.Float).SetInt(amount),
		big.NewFloat(1e18),
	)

	 return SwapResult{
        Success:     true,
        WalletIndex: walletIndex,
        Cycle:       cycle,
        Direction:   "WMON_to_MON",
        TxHash:      signedTx.Hash().Hex(),
        Fee:         fmt.Sprintf("%.6f MON", feeStr),
        Amount:      fmt.Sprintf("%.4f WMON", amountFloat),
    }
}

func processSwapResults(results chan SwapResult, totalswap int, wmonABI abi.ABI) {
	successCount := 0

	for res := range results {
		if res.Success {
			successCount++
			fmt.Printf("%s %s %s\n", green(fmt.Sprintf("[Wallet #%d]", res.WalletIndex)), cyan(res.Direction), green(fmt.Sprintf("(Cycle %d/%d)", res.Cycle, totalswap)))
			fmt.Printf("%s %s\n", yellow("Amount:"), magenta(res.Amount))
			fmt.Printf("%s %s\n", yellow("TxHash:"), blue(shortenHash(res.TxHash)))
			fmt.Printf("%s %s\n", yellow("Fee:"), magenta(res.Fee))
			fmt.Printf("%s %s%s\n\n", yellow("Explorer:"), blue(EXPLORER_BASE_MONAD), blue(res.TxHash))

			client, err := ethclient.Dial(RPC_URL_MONAD)
			if err == nil {
				pk, err := crypto.HexToECDSA(strings.TrimPrefix(getPrivateKey(res.WalletIndex), "0x"))
				if err == nil {
					address := crypto.PubkeyToAddress(pk.PublicKey)
					monBalance, _ := getMONBalance(client, address)
					wmonBalance, _ := getWMONBalance(client, address, wmonABI)

					monBalanceFloat := new(big.Float).Quo(new(big.Float).SetInt(monBalance), big.NewFloat(1e18))
					wmonBalanceFloat := new(big.Float).Quo(new(big.Float).SetInt(wmonBalance), big.NewFloat(1e18))

					fmt.Printf("%s %s %s MON | %s WMON\n\n",
						green("New Balance:"),
						cyan("=>"),
						magenta(fmt.Sprintf("%.4f", monBalanceFloat)),
						magenta(fmt.Sprintf("%.4f", wmonBalanceFloat)))
					fmt.Println("▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔")
				}
				client.Close()
			}

		} else {
			fmt.Printf("\n%s %s\n", red("❌ SWAP FAILED"), yellow(fmt.Sprintf("[Wallet #%d]", res.WalletIndex)))
			fmt.Printf("%s %v\n", red("Error:"), res.Error)
			fmt.Printf("%s %d/%d\n", yellow("Total successfully swap:"), successCount, totalswap)
			return
		}
	}

	fmt.Println(green("\n✅ SWAP SUCCESS"))
	fmt.Println("Follow X : 0xNekowawolf\n")
	fmt.Printf("%s %d/%d\n", yellow("Total successfully swap:"), successCount, totalswap)
}

func getPrivateKey(walletIndex int) string {
    keys := getPrivateKeys()
    if walletIndex-1 < len(keys) {
        return keys[walletIndex-1]
    }
    return ""
}

func getPrivateKeys() []string {
    wallets := make([]string, 20)
    for i := 0; i < 20; i++ {
        wallets[i] = strings.TrimSpace(os.Getenv(fmt.Sprintf("PRIVATE_KEYS_WALLET%d", i+1)))
    }

    var activeWallets []string
    for _, key := range wallets {
        if key != "" {
            activeWallets = append(activeWallets, key)
        }
    }
    return activeWallets
}

func shortenHash(hash string) string {
	if len(hash) < 16 {
		return hash
	}
	return hash[:8] + "..." + hash[len(hash)-8:]
}

func ShowInitialBalances() {
	godotenv.Load()

	wallets := make([]string, 20)
	for w := 0; w < 20; w++ {
		wallets[w] = os.Getenv(fmt.Sprintf("PRIVATE_KEYS_WALLET%d", w+1))
	}

	var activeWallets []string
	for _, key := range wallets {
		if key != "" {
			activeWallets = append(activeWallets, key)
		}
	}

	if len(activeWallets) == 0 {
		fmt.Println("No active wallets found")
		return
	}

	wmonABI, err := loadWMonABI()
	if err != nil {
		fmt.Println("Error loading ABI:", err)
		return
	}

	fmt.Println("\nInitial Balances:")
	for w, key := range activeWallets {
		pk, err := crypto.HexToECDSA(strings.TrimPrefix(key, "0x"))
		if err != nil {
			continue
		}

		address := crypto.PubkeyToAddress(pk.PublicKey)
		client, err := ethclient.Dial(RPC_URL_MONAD)
		if err != nil {
			continue
		}

		monBalance, _ := getMONBalance(client, address)
		wmonBalance, _ := getWMONBalance(client, address, wmonABI)
		client.Close()

		monBalanceFloat := new(big.Float).Quo(new(big.Float).SetInt(monBalance), big.NewFloat(1e18))
		wmonBalanceFloat := new(big.Float).Quo(new(big.Float).SetInt(wmonBalance), big.NewFloat(1e18))

		fmt.Printf("[Wallet #%d] Balance: %s MON | %s WMON\n",
			w+1, magenta(fmt.Sprintf("%.4f", monBalanceFloat)), magenta(fmt.Sprintf("%.4f", wmonBalanceFloat)))
	}
}
