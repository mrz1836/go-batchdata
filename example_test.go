package batchdata_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/mrz1836/go-batchdata"
)

// The application decides where the key comes from; the SDK reads no
// environment. Printing a client never shows the key.
func ExampleNewClient() {
	apiKey := "EXAMPLE-KEY-0000" // for example, from a secret manager
	client, err := batchdata.NewClient(apiKey,
		batchdata.WithTimeout(45*time.Second),
		batchdata.WithUserAgent("example-app/1.0"),
	)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(client)
	// Output: batchdata.Client{timeout: 45s, maxResponseBytes: 8388608}
}

// An invalid option fails before any call, and the error names the option
// without repeating its value.
func ExampleNewClient_invalidOption() {
	_, err := batchdata.NewClient("EXAMPLE-KEY-0000", batchdata.WithTimeout(0))
	fmt.Println(errors.Is(err, batchdata.ErrInvalidConfig))
	fmt.Println(err)
	// Output:
	// true
	// batchdata: invalid configuration: WithTimeout: must be positive
}
