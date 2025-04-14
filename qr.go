package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/markusmobius/go-dateparser"
)

const (
	gray   = "\033[38;5;242m"
	yellow = "\033[33m"
	blue   = "\033[1;34m"
	green  = "\033[1;32m"
	bold   = "\033[1m"
	reset  = "\033[0m"
)

// Command definitions
type Command struct {
	Aliases     string
	Description string
	Usage       string
	Example     string
}

var commands = map[string]Command{
	"list": {
		Aliases:     "l|list",
		Description: "List all reminders",
		Usage:       "",
		Example:     "",
	},
	"add": {
		Aliases:     "a|add",
		Description: "Add a new reminder",
		Usage:       "<reminder text> [date/time]",
		Example:     "",
	},
	"remove": {
		Aliases:     "d|r|del|remove",
		Description: "Remove a reminder by index or text",
		Usage:       "<index|text>",
		Example:     "",
	},
}

type Reminder struct {
	ID   string
	Text string
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "l", "list":
		listReminders()
	case "a", "add":
		if len(os.Args) < 3 {
			fmt.Println("Error: No reminder text provided")
			printUsage()
			os.Exit(1)
		}
		reminderText := strings.Join(os.Args[2:], " ")
		addReminder(reminderText)
	case "d", "r", "del", "remove":
		if len(os.Args) < 3 {
			fmt.Println("Error: No reminder index or text provided")
			printUsage()
			os.Exit(1)
		}
		target := strings.Join(os.Args[2:], " ")
		removeReminder(target)
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func listReminders() {
	cmd := exec.Command("reminders", "show", "Sooner")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error listing reminders: %v\n", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		index := strings.TrimSpace(parts[0])
		content := strings.TrimSpace(parts[1])

		timeInd := ""
		if idx := strings.LastIndex(content, "("); idx != -1 {
			timeInd = content[idx:]
			content = strings.TrimSpace(content[:idx])
		}

		fmt.Printf("%s[%s%s%s]%s %s", gray, yellow, index, gray, reset, content)

		if timeInd != "" {
			timeText := timeInd[1 : len(timeInd)-1]
			fmt.Printf(" %s(%s%s%s)%s", gray, blue, timeText, gray, reset)
		}
		fmt.Println()
	}
}

func addReminder(text string) {
	// Parse the date
	_, dt, err := dateparser.Search(nil, text)

	var reminderCmd *exec.Cmd
	var parsedTime time.Time

	if len(dt) > 0 {
		result := dt[0].Date
		dateText := dt[0].Text

		cleanText := strings.TrimSpace(strings.Replace(text, dateText, "", 1))

		if err != nil || !result.Time.After(time.Now()) {
			reminderCmd = exec.Command("reminders", "add", "Sooner", text)
		} else {
			parsedTime = result.Time
			dateTime := parsedTime.Format("2006-01-02 15:04:05")

			reminderCmd = exec.Command("reminders", "add", "Sooner", cleanText, "-d", dateTime)
		}

		reminderCmd.Stdout = nil
		reminderCmd.Stderr = os.Stderr

		if err := reminderCmd.Run(); err != nil {
			fmt.Printf("Error adding reminder: %v\n", err)
			os.Exit(1)
		} else {
			fmt.Printf("Added %s'%s'%s", gray, green+cleanText+gray, reset)
			if !parsedTime.IsZero() {
				fmt.Printf(" %s(parsed as: %s%s%s)%s",
					gray,
					blue,
					parsedTime.Format("Mon Jan 2 15:04:05"),
					gray,
					reset)
			}
			fmt.Println()
		}
	} else {
		reminderCmd = exec.Command("reminders", "add", "Sooner", text)
		reminderCmd.Stdout = nil
		reminderCmd.Stderr = os.Stderr

		if err := reminderCmd.Run(); err != nil {
			fmt.Printf("Error adding reminder: %v\n", err)
			os.Exit(1)
		} else {
			fmt.Printf("Added %s'%s'%s", gray, green+text+gray, reset)
			if !parsedTime.IsZero() {
				fmt.Printf(" %s(parsed as: %s%s%s)%s",
					gray,
					blue,
					parsedTime.Format("Mon Jan 2 15:04:05"),
					gray,
					reset)
			}
			fmt.Println()
		}

	}
}

func removeReminder(target string) {
	reminders, err := getReminders()
	if err != nil {
		fmt.Printf("Error getting reminders: %v\n", err)
		os.Exit(1)
	}

	if len(reminders) == 0 {
		fmt.Println("No reminders found")
		return
	}

	if index, err := strconv.Atoi(target); err == nil {
		if index >= 0 && index < len(reminders) {
			removeReminderByID(reminders[index].ID)
			return
		}
		fmt.Printf("Invalid index: %d. Valid range is 0-%d\n", index, len(reminders)-1)
		return
	}

	bestMatch := ""
	bestScore := 0.0
	bestIndex := -1

	for i, reminder := range reminders {
		distance := fuzzy.LevenshteinDistance(target, reminder.Text)
		maxLen := float64(max(len(target), len(reminder.Text)))
		normalizedScore := 1.0 - (float64(distance) / maxLen)

		if normalizedScore > bestScore {
			bestScore = normalizedScore
			bestMatch = reminder.Text
			bestIndex = i
		}
	}

	if bestScore < 0.50 {
		fmt.Printf("No reminder found matching '%s' with at least 50%% similarity\n", target)
		return
	}

	fmt.Printf("Removing reminder: %s (%.0f%% match)\n", bestMatch, bestScore*100)
	removeReminderByID(reminders[bestIndex].ID)
}

func getReminders() ([]Reminder, error) {
	cmd := exec.Command("reminders", "show", "Sooner")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var reminders []Reminder
	scanner := bufio.NewScanner(&out)
	re := regexp.MustCompile(`^(\d+): (.+?)( \(in .+\))?$`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 3 {
			reminders = append(reminders, Reminder{
				ID:   matches[1],
				Text: strings.TrimSuffix(matches[2], " "),
			})
		}
	}

	return reminders, nil
}

func removeReminderByID(id string) {
	cmd := exec.Command("reminders", "delete", "Sooner", id)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error removing reminder: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	for _, cmd := range commands {
		usage := cmd.Usage
		if usage != "" {
			usage = " " + usage
		}
		fmt.Printf("  qr %s%s\t%s\n", cmd.Aliases, usage, cmd.Description)
	}

	fmt.Println("\nExamples:")
	for _, cmd := range commands {
		if cmd.Example != "" {
			for _, example := range strings.Fields(cmd.Example) {
				fmt.Printf("  qr %s %s\n", strings.Split(cmd.Aliases, "|")[0], example)
			}
		}
	}
	fmt.Println()
}
