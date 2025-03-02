package main

import (
	"bufio"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func loadPrivateKey(filename string) (ed25519.PrivateKey, error) {
	privKeyBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(privKeyBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	edPriv, ok := privKey.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an Ed25519 private key")
	}
	return edPriv, nil
}

func main() {
	// Load the private key
	privKey, err := loadPrivateKey("privkey.pem")
	if err != nil {
		fmt.Println("Error loading private key:", err)
		os.Exit(1)
	}

	// Define Lipgloss styles
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	outputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	// Instructions for the user
	fmt.Println("Enter challenges one by one. Type 'quit' to exit.")

	// Set up scanner for reading input
	scanner := bufio.NewScanner(os.Stdin)

	// Main loop to continuously accept challenges
	for {
		// Print styled prompt
		fmt.Print(promptStyle.Render("Enter challenge (base64): "))

		// Read input line
		if !scanner.Scan() {
			break // Exit on EOF (e.g., Ctrl+D or Ctrl+Z)
		}

		// Process the input
		challengeB64 := strings.TrimSpace(scanner.Text())
		if challengeB64 == "" {
			continue // Skip empty lines
		}
		if challengeB64 == "quit" {
			break // Exit on 'quit'
		}

		// Decode the base64 challenge
		challenge, err := base64.StdEncoding.DecodeString(challengeB64)
		if err != nil {
			fmt.Println(outputStyle.Render("Error: invalid base64"))
			continue
		}

		// Sign the challenge
		signature := ed25519.Sign(privKey, challenge)
		sigB64 := base64.StdEncoding.EncodeToString(signature)

		// Print the signature
		fmt.Println(outputStyle.Render("Signature: " + sigB64))
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading input:", err)
	}
}
