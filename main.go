package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
)

var opts struct {
	Threads       int    // Number of threads to use
	ResolverIP    string // IP of the DNS resolver to use for lookups
	Protocol      string // Protocol to use for lookups
	Port          uint16 // Port to use for lookups
	Domain        bool   // Output only domains
	ResolversFile string // Path to a text file containing custom resolvers
}

func main() {
	parseArgs()

	// default of 8 threads
	numWorkers := opts.Threads

	work := make(chan string)
	go func() {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			work <- s.Text()
		}
		close(work)
	}()

	wg := &sync.WaitGroup{}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go doWork(work, wg)
	}
	wg.Wait()
}

func parseArgs() {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-t":
			opts.Threads = parseIntArg(args, i)
			i++
		case "-r":
			opts.ResolverIP = args[i+1]
			i++
		case "-P":
			opts.Protocol = args[i+1]
			i++
		case "-p":
			opts.Port = parseUint16Arg(args, i)
			i++
		case "-d":
			opts.Domain = true
		case "-f":
			opts.ResolversFile = args[i+1]
			i++
		}
	}
}

func parseIntArg(args []string, index int) int {
	if index+1 < len(args) {
		val := args[index+1]
		num, err := strconv.Atoi(val)
		if err == nil {
			return num
		}
	}
	return 0
}

func parseUint16Arg(args []string, index int) uint16 {
	if index+1 < len(args) {
		val := args[index+1]
		num, err := strconv.ParseUint(val, 10, 16)
		if err == nil {
			return uint16(num)
		}
	}
	return 0
}

func doWork(work chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	var resolvers []string

	if opts.ResolverIP != "" {
		resolvers = append(resolvers, fmt.Sprintf("%s:%d", opts.ResolverIP, opts.Port))
	}

	if opts.ResolversFile != "" {
		file, err := os.Open(opts.ResolversFile)
		if err != nil {
			fmt.Println("Failed to open resolvers file:", err)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			resolver := strings.TrimSpace(scanner.Text())
			if resolver != "" {
				resolvers = append(resolvers, resolver)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Failed to read resolvers file:", err)
			return
		}
	}

	for ip := range work {
		for _, resolver := range resolvers {
			addr, err := lookupAddrWithContext(context.Background(), ip, resolver)
			if err != nil {
				continue
			}

			for _, a := range addr {
				if opts.Domain {
					fmt.Println(strings.TrimRight(a, "."))
				} else {
					fmt.Println(ip, "\t", a)
				}
			}
		}
	}
}

// Custom lookupAddrWithContext function that uses the specified resolver
func lookupAddrWithContext(ctx context.Context, ip, resolver string) ([]string, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, opts.Protocol, resolver)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Perform DNS lookup using the connection
	return net.LookupAddr(ip)
}
