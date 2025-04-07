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
	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
)

const (
	gray   = "\033[38;5;242m"
	yellow = "\033[33m"
	blue   = "\033[1;34m"
	green  = "\033[1;32m"
	bold   = "\033[1m"
	reset  = "\033[0m"
)

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
	case "r", "remove":
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

		// Split by colon to separate index and content
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		index := strings.TrimSpace(parts[0])
		content := strings.TrimSpace(parts[1])

		// Check for time indication at the end
		timeInd := ""
		if idx := strings.LastIndex(content, "("); idx != -1 {
			timeInd = content[idx:]
			content = strings.TrimSpace(content[:idx])
		}

		// Format the output with colors
		fmt.Printf("%s[%s%s%s]%s %s", gray, yellow, index, gray, reset, content)

		if timeInd != "" {
			timeText := timeInd[1 : len(timeInd)-1] // remove parentheses
			fmt.Printf(" %s(%s%s%s)%s", gray, blue, timeText, gray, reset)
		}
		fmt.Println()
	}
}

func addReminder(text string) {
	// Parse date and time from the text
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)

	now := time.Now()
	result, err := w.Parse(text, now)

	var reminderCmd *exec.Cmd

	if err != nil || result == nil {
		// No date/time found, add as a regular reminder
		reminderCmd = exec.Command("reminders", "add", "Sooner", text)
	} else {
		// Extract the date/time and the actual reminder text
		dateTime := result.Time.Format("2006-01-02 15:04:05")

		// Remove the date/time part from the reminder text
		cleanText := text[:result.Index] + text[result.Index+len(result.Text):]
		cleanText = strings.TrimSpace(cleanText)

		// If the text is empty after removing the date, use a generic message
		if cleanText == "" {
			cleanText = "Reminder"
		}

		// Add reminder with the specified date and time
		reminderCmd = exec.Command("reminders", "add", "Sooner", cleanText, "-d", dateTime)
	}

	reminderCmd.Stdout = nil
	reminderCmd.Stderr = os.Stderr

	if err := reminderCmd.Run(); err != nil {
		fmt.Printf("Error adding reminder: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Printf("Added %s'%s'%s\n",
			gray, green+text+gray,
			reset)
	}
}

func removeReminder(target string) {
	// Get the list of reminders
	reminders, err := getReminders()
	if err != nil {
		fmt.Printf("Error getting reminders: %v\n", err)
		os.Exit(1)
	}

	if len(reminders) == 0 {
		fmt.Println("No reminders found")
		return
	}

	// Check if target is a number (index)
	if index, err := strconv.Atoi(target); err == nil {
		if index >= 0 && index < len(reminders) {
			// Valid index, remove the reminder
			removeReminderByID(reminders[index].ID)
			return
		}
		fmt.Printf("Invalid index: %d. Valid range is 0-%d\n", index, len(reminders)-1)
		return
	}

	// Target is not a number, try to find the closest match
	bestMatch := ""
	bestScore := 0.0
	bestIndex := -1

	for i, reminder := range reminders {
		// Calculate similarity score using fuzzy matching
		distance := fuzzy.LevenshteinDistance(target, reminder.Text)
		maxLen := float64(max(len(target), len(reminder.Text)))
		normalizedScore := 1.0 - (float64(distance) / maxLen)

		if normalizedScore > bestScore {
			bestScore = normalizedScore
			bestMatch = reminder.Text
			bestIndex = i
		}
	}

	// If the best match is less than 75% similar, discard it
	if bestScore < 0.75 {
		fmt.Printf("No reminder found matching '%s' with at least 75%% similarity\n", target)
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

	// Parse the output to extract reminders
	var reminders []Reminder
	scanner := bufio.NewScanner(&out)

	// Regular expression to match reminder lines with IDs
	// This regex pattern might need adjustment based on the actual output format
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
	fmt.Println("  qr l|list                                List all reminders")
	fmt.Println("  qr a|add <reminder text> [date/time]     Add a new reminder")
	fmt.Println("  qr r|remove <index|text>                 Remove a reminder by index or text")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  qr a Buy groceries")
	fmt.Println("  qr a Call mom tomorrow at 5pm")
	fmt.Println("  qr r 2                                   Remove reminder at index 2")
	fmt.Println("  qr r Buy groceries                       Remove reminder matching text")
}
