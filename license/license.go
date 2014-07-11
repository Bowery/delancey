// Copyright 2014 Bowery, Inc.
package main

import (
	"code.google.com/p/go-uuid/uuid"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	months int
	years  int
	check  bool
)

func init() {
	flag.IntVar(&months, "months", 0, "The number of months the license should last.")
	flag.IntVar(&months, "m", 0, "The number of months the license should last.")
	flag.IntVar(&years, "years", 0, "The number of years the license should last.")
	flag.IntVar(&years, "y", 0, "The number of years the license should last.")
	flag.BoolVar(&check, "check", false, "Check a keys user limit and expiration.")
	flag.BoolVar(&check, "c", false, "Check a keys user limit and expiration.")
}

func main() {
	flag.Parse()
	args := flag.Args()

	if check && len(args) <= 0 {
		fmt.Fprintln(os.Stderr, "A key to check is required.")
		os.Exit(1)
	}

	if !check && (len(args) <= 0 || (months <= 0 && years <= 0)) {
		fmt.Fprintln(os.Stderr, "A user limit and (future) expire date is required.")
		os.Exit(1)
	}

	if !check {
		key, err := genKey(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fmt.Println(key)
		return
	}

	expired, limit, err := checkKey(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Invalid key input")
		os.Exit(1)
	}

	fmt.Println(expired, limit)
}

// genKey creates a key from a given user limit and the months/years
// expiration duration.
func genKey(userLimit string) (string, error) {
	// Convert limit * 1000 to base 16.
	limit, err := strconv.Atoi(userLimit)
	if err != nil {
		return "", err
	}
	if limit <= -1 {
		return "", errors.New("User limit must at least 1.")
	}
	limitStr := strconv.FormatInt(int64(limit*1000), 16)

	// Calculate date license expires and convert to unix time string.
	monthDur := (time.Hour * 24) * time.Duration(30*months)
	yearDur := (time.Hour * 24) * time.Duration(365*years)
	expire := time.Now().Add(monthDur + yearDur)
	expireStr := strconv.FormatInt(expire.Unix(), 10)

	// Generate limit len with padding.
	license := []byte(strconv.Itoa(len(limitStr)) + "x")
	if len(license)%2 == 0 {
		license = append(license, 'x')
	}

	// Add a uuid to end of the limit string.
	limitStr += "r" + strings.Join(strings.Split(uuid.New(), "-"), "")

	// Create sequence code and write even expire bytes, odd license bytes.
	seq := make([]byte, len(expireStr)*2)
	e := 0
	l := 0
	for i := range seq {
		// Write expire bytes.
		if i%2 == 0 {
			seq[i] = expireStr[e]
			e++
		} else {
			seq[i] = limitStr[l]
			l++
		}
	}
	seq = append(seq, 'x')
	license = append(license, seq...)

	// Add needed padding to create multiple of 6.
	for len(license)%6 != 0 {
		license = append(license, 'x')
	}

	// Split on 6 bytes.
	final := make([]byte, 0)
	i := 0
	for _, c := range license {
		if i > 5 {
			i = 0
			final = append(final, '-')
		}

		final = append(final, c)
		i++
	}

	return string(final), nil
}

// checkKey checks if the key is expired and the number of users it's
// limited to.
func checkKey(key string) (bool, int, error) {
	var tmp string
	var err error
	key = strings.Join(strings.Split(key, "-"), "")
	limitLen := 0

	// Find the limit string length, and trim off padding.
	for i, c := range key {
		if c == 'x' && (len(tmp)+1%2 != 0 || tmp[len(tmp)-1] == 'x') {
			limitLen, err = strconv.Atoi(tmp)
			if err != nil {
				return false, 0, err
			}
			key = strings.TrimLeft(key[i:], "x")
			break
		}

		tmp += string(c)
	}

	// Extract user limit and expire date.
	expireStr, limitStr := "", ""
	for i, c := range key {
		if c == 'x' {
			break
		}

		if i%2 == 0 {
			expireStr += string(c)
		} else if len(limitStr) < limitLen {
			limitStr += string(c)
		}
	}

	// Parse user limit and expire date.
	expireInt, err := strconv.ParseInt(expireStr, 10, 64)
	if err != nil {
		return false, 0, err
	}
	expire := time.Unix(expireInt, 0)

	limit, err := strconv.ParseInt(limitStr, 16, 64)
	if err != nil {
		return false, 0, err
	}
	limit /= 1000

	return time.Now().After(expire), int(limit), nil
}
