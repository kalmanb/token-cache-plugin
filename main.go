package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const tokenCacheFilename = ".token-cache"

func main() {
	config, err := readConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading configuration: %s\n", err)
		os.Exit(1)
	}

	// Attempt to read a previously cached token before prompting for a username and password.
	// If this doesn't work then attempt to acquire a new token.
	token, err := ioutil.ReadFile(tokenCacheFilename)
	if err != nil {
		username, password := readCredentials()
		if token, err = requestToken(config, username, password); err != nil {
			fmt.Fprintf(os.Stderr, "Error requesting token: %s\n", err)
			os.Exit(1)
		}
	}

	// Write token to cache file to be used for next time.
	if config.cacheTokens == true {
		err := ioutil.WriteFile(tokenCacheFilename, token, os.FileMode(0600))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to cache token locally: %s\n", err)
		}
	}

	// Write token to stdout to be used by kubectl.
	if err = outputToken(token); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to output token: %s\n", err)
		os.Exit(1)
	}
}

func readCredentials() (username, password string) {
	fmt.Fprintf(os.Stderr, "Please enter username: \n")
	fmt.Fscanf(os.Stdin, "%s", &username)

	fmt.Fprintf(os.Stderr, "Please enter password: \n")
	fmt.Fscanf(os.Stdin, "%s", &password)

	return strings.TrimSpace(username), strings.TrimSpace(password)
}

func requestToken(config Config, username, password string) (token []byte, err error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.skipTLSVerification,
			},
		},
	}

	req, err := http.NewRequest("GET", config.tokenServerURL, nil)
	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	if err != nil {
		return token, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func outputToken(token []byte) error {
	execCredential := ExecCredential{
		APIVersion: "client.authentication.k8s.io/v1alpha1",
		Kind:       "ExecCredential",
		Status: map[string]string{
			"token": string(token),
		},
	}

	output, err := json.Marshal(execCredential)
	if err != nil {
		return err
	}

	_, err = fmt.Printf("%s", output)
	return err
}

func readConfig() (config Config, err error) {
	config.tokenServerURL = os.Getenv("TOKEN_SERVER_URL")
	if config.tokenServerURL == "" {
		return config, errors.New("TOKEN_SERVER_URL not specified")
	}

	cacheTokens := os.Getenv("CACHE_TOKENS")
	if cacheTokens == "" {
		cacheTokens = "false"
	}
	if config.cacheTokens, err = strconv.ParseBool(cacheTokens); err != nil {
		return config, errors.New("Invalid value specified for CACHE_TOKENS")
	}

	config.caCert = os.Getenv("CA_CERT")
	skipTLSVerification := os.Getenv("SKIP_TLS_VERIFICATION")
	if skipTLSVerification == "" {
		skipTLSVerification = "false"
	}
	if config.skipTLSVerification, err = strconv.ParseBool(skipTLSVerification); err != nil {
		return config, errors.New("Invalid value specified for SKIP_TLS_VERIFICATION")
	}

	return config, nil
}
