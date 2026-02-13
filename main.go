package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"hme-cli/icloud"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

var (
	ErrCookieFileEmpty = errors.New("cookie file exists but is empty")
)

func ensureCookieFile(path string) (string, error) {
	info, err := os.Stat(path)

	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", fmt.Errorf("failed to create config directory: %w", err)
		}

		return "", fmt.Errorf("cookie file missing and was created, copy cookies from icloud.com to %s. Please check README.md for details", path)
	} else if err != nil {
		return "", fmt.Errorf("cookie file access error: %w", err)
	}

	mode := info.Mode()
	if mode.Perm()&0077 != 0 {
		fmt.Printf("unsafe permissions detected (%o) for %s. fixing to 0600...\n", mode.Perm(), filepath.Base(path))
		if err := os.Chmod(path, 0600); err != nil {
			return "", fmt.Errorf("failed to fix permissions: %w", err)
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	cookieStr := strings.TrimSpace(string(content))
	if cookieStr == "" {
		return "", ErrCookieFileEmpty
	}

	return cookieStr, nil
}

func getCookies() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}
	configPath := filepath.Join(homeDir, ".config", "hme-cli", "cookies.txt")

	rawCookies, err := ensureCookieFile(configPath)
	if err != nil {
		return "", err
	}

	return rawCookies, nil
}

func getCachePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine cache directory: %w", err)
	}
	return filepath.Join(cacheDir, "hme-cli", "latest"), nil
}

func getCache() ([]byte, error) {
	path, err := getCachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cache missing or unavailable: %w", err)
	}

	return data, nil
}

func createCache(data []byte) error {
	path, err := getCachePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cache write error: %w", err)
	}

	return nil
}

func getAnonymousIds(c *cli.Command) ([]string, error) {
	emailsToProcess := make(map[string]bool)

	for _, arg := range c.Args().Slice() {
		if clean := strings.TrimSpace(arg); clean != "" {
			emailsToProcess[clean] = true
		}
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				emailsToProcess[line] = true
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("stdin read error: %w", err)
		}
	}

	keys := make([]string, 0, len(emailsToProcess))

	for email := range emailsToProcess {
		keys = append(keys, email)
	}

	return keys, nil
}

func main() {
	rawCookies, err := getCookies()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ic, err := icloud.NewClient(rawCookies)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error initializing client: %v\n", err)
		os.Exit(1)
	}

	cmd := &cli.Command{
		Commands: []*cli.Command{
			{
				Name:    "new",
				Aliases: []string{"n"},
				Usage:   "generate and reserve new email/s",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "count",
						Aliases: []string{"c"},
						Usage:   "count of emails to generate",
						Value:   1,
					},
					&cli.StringFlag{
						Name:    "label",
						Aliases: []string{"l"},
						Usage:   "label to be used, random string by default",
						Value:   "",
					},
					&cli.StringFlag{
						Name:    "note",
						Aliases: []string{"n"},
						Usage:   "note to be used, empty by default",
						Value:   "",
					},
					&cli.IntFlag{
						Name:    "interval",
						Aliases: []string{"i"},
						Usage:   "interval between email generations in ms",
						Value:   3000,
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					count := c.Int("count")
					if count < 1 {
						return fmt.Errorf("count must be at least 1")
					}

					label := c.String("label")
					note := c.String("note")
					interval_ms := c.Int("interval")
					if interval_ms < 0 {
						interval_ms = 0
					}

					for i := range count {
						currentLabel := label
						if currentLabel == "" {
							currentLabel = RandomString(12)
						}

						email, err := ic.GenerateOne(currentLabel, note)
						if err != nil {
							return err
						}

						fmt.Println(email)

						if i < count-1 {
							time.Sleep(time.Duration(interval_ms) * time.Millisecond)
						}
					}

					return nil
				},
			},
			{
				Name:    "deactivate",
				Aliases: []string{"d"},
				Usage:   "deactivate an email",
				Action: func(ctx context.Context, c *cli.Command) error {
					ids, err := getAnonymousIds(c)
					if err != nil {
						return err
					}

					if len(ids) == 0 {
						return fmt.Errorf("anonymous ids not provided")
					}

					fmt.Printf("deactivating %d addresses...\n", len(ids))

					for _, id := range ids {
						fmt.Printf("deactivating: %s ... ", id)

						err := ic.DeactivateEmail(id)

						if err != nil {
							return err
						} else {
							fmt.Printf("OK\n")
						}
					}

					return nil
				},
			},
			{
				Name:    "reactivate",
				Aliases: []string{"a"},
				Usage:   "reactivate an email",
				Action: func(ctx context.Context, c *cli.Command) error {
					ids, err := getAnonymousIds(c)
					if err != nil {
						return err
					}

					if len(ids) == 0 {
						return fmt.Errorf("anonymous ids not provided")
					}

					fmt.Printf("reactivating %d addresses...\n", len(ids))

					for _, id := range ids {
						fmt.Printf("reactivating: %s ... ", id)

						err := ic.ReactivateEmail(id)

						if err != nil {
							return err
						} else {
							fmt.Printf("OK\n")
						}
					}

					return nil
				},
			},
			{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "list emails",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "cache",
						Aliases: []string{"c"},
						Usage:   "returns latest result from cache",
						Value:   false,
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					useCache := c.Bool("cache")

					if useCache {
						cachedData, err := getCache()
						if err == nil && len(cachedData) > 0 {
							os.Stdout.Write(cachedData)
							return nil
						}
					}

					emailList, err := ic.ListEmails()
					if err != nil {
						return err
					}

					var buf bytes.Buffer
					for i, email := range emailList.Result.HmeEmails {
						status := "INACTIVE"
						if email.IsActive {
							status = "ACTIVE"
						}

						buf.WriteString(fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s\n",
							i+1,
							email.AnonymousID,
							email.Label,
							email.Hme,
							FormatUnixMilliTime(email.CreateTimestamp),
							status,
						))
					}

					out := buf.Bytes()
					os.Stdout.Write(out)

					if err := createCache(out); err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to save cache: %v\n", err)
					}

					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
