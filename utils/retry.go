package utils

import (
	"fmt"
	"net/http"
	"time"
)

func WithRetry[T any, R any](f func(...T) (R, error), errorMessage string) func(...T) (R, error) {
	return func(args ...T) (R, error) {
		count := 10
		var zero R
		var err error
		for i := 1; i <= count; i++ {
			resp, err := f(args...)
			if err == nil {
				return resp, nil
			}

			if len(errorMessage) > 0 {
				fmt.Printf("%s %v, attempt: %d\n", errorMessage, err, i)
			}
			time.Sleep(5 * time.Second)

		}
		return zero, err
	}
}

func HTTPWithRetry(f func(string) (*http.Response, error), url string) (*http.Response, error) {
	count := 10
	var resp *http.Response
	var err error
	for i := 0; i < count; i++ {
		resp, err = f(url)
		if err != nil {
			fmt.Printf("Error calling url %v\n", url)
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	return resp, err
}
