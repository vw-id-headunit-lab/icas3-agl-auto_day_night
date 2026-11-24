package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

func exchange(socket string, arguments ...string) (string, error) {
	if len(arguments) != 4 {
		return "", errors.New("attach command requires four fields")
	}
	for _, field := range arguments {
		if strings.ContainsRune(field, 0) {
			return "", errors.New("NUL in attach argument")
		}
	}
	c, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return "", err
	}
	defer c.Close()
	if err = c.SetDeadline(time.Now().Add(60 * time.Second)); err != nil {
		return "", err
	}
	payload := "1\x00" + strings.Join(arguments, "\x00") + "\x00"
	if _, err = io.WriteString(c, payload); err != nil {
		return "", err
	}
	r := bufio.NewReader(c)
	status, err := r.ReadSlice('\n')
	if err != nil {
		return "", err
	}
	statusCode := strings.TrimSpace(string(status))
	const limit = 1024 * 1024
	result, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return "", err
	}
	if len(result) > limit {
		return "", errors.New("attach response exceeds size limit")
	}
	if statusCode != "0" {
		return "", fmt.Errorf("attach rejected: %s %s", statusCode, result)
	}
	return string(result), nil
}
