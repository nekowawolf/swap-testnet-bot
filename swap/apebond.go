package swap

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind" 
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

const (
	WMON_CONTRACT_ADDRESS = "0x760AfE86e5de5fa0Ee542fc7B7B713e1c5425701"
	RPC_URL_MONAD       = "https://testnet-rpc.monad.xyz"
	CHAIN_ID_MONAD      = 10143
	GAS_LIMIT_MONAD     = 150000
	GAS_PRICE_MONAD     = 50000000000
	EXPLORER_BASE_MONAD = "https://testnet.monadexplorer.com/tx/"
	DELAY_SECONDS_MONAD = 2
)

type SwapResult struct {
	Success      bool
	WalletIndex  int
	Direction    string 
	TxHash       string
	Fee          string
	Amount       string
	Error        error
}

func loadWMonABI() (abi.ABI, error) {
	abiJSON := `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"src","type":"address"},{"indexed":true,"internalType":"address","name":"guy","type":"address"},{"indexed":false,"internalType":"uint256","name":"wad","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"dst","type":"address"},{"indexed":false,"internalType":"uint256","name":"wad","type":"uint256"}],"name":"Deposit","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"src","type":"address"},{"indexed":true,"internalType":"address","name":"dst","type":"address"},{"indexed":false,"internalType":"uint256","name":"wad","type":"uint256"}],"name":"Transfer","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"src","type":"address"},{"indexed":false,"internalType":"uint256","name":"wad","type":"uint256"}],"name":"Withdrawal","type":"event"},{"stateMutability":"payable","type":"fallback"},{"inputs":[{"internalType":"address","name":"","type":"address"},{"internalType":"address","name":"","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"guy","type":"address"},{"internalType":"uint256","name":"wad","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"decimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"deposit","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"dst","type":"address"},{"internalType":"uint256","name":"wad","type":"uint256"}],"name":"transfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"src","type":"address"},{"internalType":"address","name":"dst","type":"address"},{"internalType":"uint256","name":"wad","type":"uint256"}],"name":"transferFrom","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"wad","type":"uint256"}],"name":"withdraw","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
	return abi.JSON(strings.NewReader(abiJSON))
}

func ApebondSwap(amount *big.Int, numswap int, direction string) {
    godotenv.Load()

    wallets := make([]string, 10)
    for i := 0; i < 10; i++ {
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

    fmt.Println("\nInitial Balances:")
    for i, key := range activeWallets {
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

        fmt.Printf("[Wallet #%d] Balance: %.4f MON | %.4f WMON\n\n", 
            i+1, monBalanceFloat, wmonBalanceFloat)
    }

    results := make(chan SwapResult, numswap)
    var wg sync.WaitGroup
    walletMutexes := make([]sync.Mutex, len(activeWallets))

    amountFloat := new(big.Float).Quo(new(big.Float).SetInt(amount), big.NewFloat(1e18))
    tokenName := "WMON"
    if direction == "MON_to_WMON" {
        tokenName = "MON"
    }

    fmt.Printf("\nPreparing to perform %d swap of %.4f %s to %s\n", 
        numswap, amountFloat, tokenName, strings.Split(direction, "_")[2])
    fmt.Println("Starting swap...\n")

    for i := 0; i < numswap; i++ {
        wg.Add(1)
        walletIndex := i % len(activeWallets)
        currentDirection := direction

        if numswap > 1 && i > 0 {
            if direction == "MON_to_WMON" {
                currentDirection = "WMON_to_MON"
            } else {
                currentDirection = "MON_to_WMON"
            }
        }

        go func(swapNum int, walletIdx int, dir string) {
            defer wg.Done()
            time.Sleep(time.Duration(swapNum*DELAY_SECONDS_MONAD) * time.Second)
            walletMutexes[walletIdx].Lock()
            defer walletMutexes[walletIdx].Unlock()

            var result SwapResult
            if dir == "MON_to_WMON" {
                result = swapMONtoWMON(activeWallets[walletIdx], walletIdx+1, amount, wmonABI)
            } else {
                result = swapWMONtoMON(activeWallets[walletIdx], walletIdx+1, amount, wmonABI)
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

func swapMONtoWMON(privateKey string, walletIndex int, amount *big.Int, wmonABI abi.ABI) SwapResult {
    
    client, err := ethclient.Dial(RPC_URL_MONAD)
    if err != nil {
        return SwapResult{Error: fmt.Errorf("RPC connection failed: %v", err)}
    }
    defer client.Close()

    suggestedGasPrice, err := client.SuggestGasPrice(context.Background())
    if err != nil {
        return SwapResult{Error: fmt.Errorf("failed to get gas price: %v", err)}
    }

    pk, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
    if err != nil {
        return SwapResult{Error: fmt.Errorf("invalid private key: %v", err)}
    }

    fromAddress := crypto.PubkeyToAddress(pk.PublicKey)
    nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
    if err != nil {
        return SwapResult{Error: fmt.Errorf("nonce error: %v", err)}
    }

    auth, err := bind.NewKeyedTransactorWithChainID(pk, big.NewInt(CHAIN_ID_MONAD))
    if err != nil {
        return SwapResult{Error: fmt.Errorf("failed to create transactor: %v", err)}
    }

    auth.Nonce = big.NewInt(int64(nonce))
    auth.GasLimit = GAS_LIMIT_MONAD
    auth.GasPrice = suggestedGasPrice
    auth.Value = amount 

    wmonContract := common.HexToAddress(WMON_CONTRACT_ADDRESS)
    contract := bind.NewBoundContract(wmonContract, wmonABI, client, client, client)

    tx, err := contract.RawTransact(auth, nil)
    if err != nil {
        return SwapResult{Error: fmt.Errorf("deposit failed: %v", err)}
    }

    receipt, err := bind.WaitMined(context.Background(), client, tx)
    if err != nil {
        return SwapResult{Error: fmt.Errorf("tx mining failed: %v", err)}
    }

    fee := new(big.Float).Quo(
        new(big.Float).SetInt(new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), suggestedGasPrice)),
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
        Direction:   "MON_to_WMON",
        TxHash:      tx.Hash().Hex(),
        Fee:         fmt.Sprintf("%.6f MON", feeStr),
        Amount:      fmt.Sprintf("%.4f MON", amountFloat),
    }
}

func swapWMONtoMON(privateKey string, walletIndex int, amount *big.Int, wmonABI abi.ABI) SwapResult {

    client, err := ethclient.Dial(RPC_URL_MONAD)
    if err != nil {
        return SwapResult{Error: fmt.Errorf("RPC connection failed: %v", err)}
    }
    defer client.Close()

    suggestedGasPrice, err := client.SuggestGasPrice(context.Background())
    if err != nil {
        return SwapResult{Error: fmt.Errorf("failed to get gas price: %v", err)}
    }

    pk, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
    if err != nil {
        return SwapResult{Error: fmt.Errorf("invalid private key: %v", err)}
    }

    fromAddress := crypto.PubkeyToAddress(pk.PublicKey)
    nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
    if err != nil {
        return SwapResult{Error: fmt.Errorf("nonce error: %v", err)}
    }

    auth, err := bind.NewKeyedTransactorWithChainID(pk, big.NewInt(CHAIN_ID_MONAD))
    if err != nil {
        return SwapResult{Error: fmt.Errorf("failed to create transactor: %v", err)}
    }

    auth.Nonce = big.NewInt(int64(nonce))
    auth.GasLimit = GAS_LIMIT_MONAD
    auth.GasPrice = suggestedGasPrice
    auth.Value = big.NewInt(0)

    wmonContract := common.HexToAddress(WMON_CONTRACT_ADDRESS)
    contract := bind.NewBoundContract(wmonContract, wmonABI, client, client, client)

    tx, err := contract.Transact(auth, "withdraw", amount)
    if err != nil {
        return SwapResult{Error: fmt.Errorf("withdraw failed: %v", err)}
    }

    receipt, err := bind.WaitMined(context.Background(), client, tx)
    if err != nil {
        return SwapResult{Error: fmt.Errorf("tx mining failed: %v", err)}
    }

    fee := new(big.Float).Quo(
        new(big.Float).SetInt(new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), suggestedGasPrice)),
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
        Direction:   "WMON_to_MON",
        TxHash:      tx.Hash().Hex(),
        Fee:         fmt.Sprintf("%.6f MON", feeStr),
        Amount:      fmt.Sprintf("%.4f WMON", amountFloat),
    }
}

func processSwapResults(results chan SwapResult, totalswap int, wmonABI abi.ABI) {
    successCount := 0
    
    for res := range results {
        if res.Success {
            successCount++
            fmt.Printf("[Wallet #%d] %s\n", res.WalletIndex, res.Direction)
            fmt.Printf("Amount: %s\n", res.Amount)
            fmt.Printf("TxHash: %s\n", shortenHash(res.TxHash))
            fmt.Printf("Fee: %s\n", res.Fee)
            fmt.Printf("Explorer: %s%s\n\n", EXPLORER_BASE_MONAD, res.TxHash)

            client, err := ethclient.Dial(RPC_URL_MONAD)
            if err == nil {
                pk, err := crypto.HexToECDSA(strings.TrimPrefix(getPrivateKey(res.WalletIndex), "0x"))
                if err == nil {
                    address := crypto.PubkeyToAddress(pk.PublicKey)
                    monBalance, _ := getMONBalance(client, address)
                    wmonBalance, _ := getWMONBalance(client, address, wmonABI)
                    
                    monBalanceFloat := new(big.Float).Quo(new(big.Float).SetInt(monBalance), big.NewFloat(1e18))
                    wmonBalanceFloat := new(big.Float).Quo(new(big.Float).SetInt(wmonBalance), big.NewFloat(1e18))
                    
                    fmt.Printf("New Balance: %.4f MON | %.4f WMON\n\n", monBalanceFloat, wmonBalanceFloat)
					fmt.Println("▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔")
                }
                client.Close()
            }
            
        } else {
            fmt.Printf("\n❌ SWAP FAILED [Wallet #%d]\n", res.WalletIndex)
            fmt.Printf("Error: %v\n", res.Error)
            fmt.Printf("Total successfully swap: %d/%d\n", successCount, totalswap)
            return
        }
    }

    fmt.Println("\n✅ SWAP SUCCESS")
    fmt.Println("Follow X : 0xNekowawolf\n")
    fmt.Printf("Total successfully swap: %d/%d\n", successCount, totalswap)
}

func getPrivateKey(walletIndex int) string {
    return os.Getenv(fmt.Sprintf("PRIVATE_KEYS_WALLET%d", walletIndex))
}

func shortenHash(hash string) string {
	if len(hash) < 16 {
		return hash
	}
	return hash[:8] + "..." + hash[len(hash)-8:]
}

func ShowInitialBalances() {
    godotenv.Load()

    wallets := make([]string, 10)
    for w := 0; w < 10; w++ {
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

        fmt.Printf("[Wallet #%d] Balance: %.4f MON | %.4f WMON\n", 
            w+1, monBalanceFloat, wmonBalanceFloat)
    }
}
