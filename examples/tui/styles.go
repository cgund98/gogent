package main

import "github.com/charmbracelet/lipgloss"

var (
	statusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	userPrefixStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	assistantPrefixStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("117")).
				Bold(true)

	toolSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)
	toolDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	toolSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	toolErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	toolResultStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
)
