# Claude Desktop Multiple Accounts on Windows — Profile Switcher

**Run multiple Claude Desktop accounts side by side on one Windows PC.** This lightweight PowerShell launcher gives every Claude user its own isolated profile — separate login, settings, MCP config and data — so you can switch between work and personal Claude accounts (or several team accounts) without signing out.

> Keywords: Claude Desktop multiple accounts, Claude multi-user, Claude profile manager, switch Claude accounts, run two Claude Desktop instances, Anthropic Claude Windows, `--user-data-dir`, PowerShell launcher.

---

## Table of contents

- [Why use this](#why-use-this)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Command-line options](#command-line-options)
- [Desktop shortcuts per account](#desktop-shortcuts-per-account)
- [How it works](#how-it-works)
- [Where data is stored](#where-data-is-stored)
- [FAQ](#faq)
- [Troubleshooting](#troubleshooting)
- [Disclaimer](#disclaimer)

## Why use this

Claude Desktop signs in to one account at a time. If you have a personal Claude Pro account and a company Claude Team or Enterprise account, you normally have to log out and back in every time you switch. This tool launches each account in its **own Claude Desktop window with its own user data folder**, so all your accounts stay signed in and can run at the same time.

## Features

- 🧑‍🤝‍🧑 **Multiple Claude accounts at once** — each profile opens in its own window.
- 🔒 **Fully isolated profiles** — separate login session, cookies, settings, MCP servers and cache.
- 📋 **Simple interactive menu** — pick a user by number, add (`A`), remove (`R`), or open your main profile (`M`).
- ⚡ **Direct launch** — start a specific account from a shortcut with `-Profile`.
- 🔍 **Auto-discovery** — existing profile folders on disk are picked up automatically.
- 🕒 **Last-used tracking** — the menu shows when each profile was last opened.
- 🪶 **No install, no dependencies** — just PowerShell, which ships with Windows.
- 🏪 Works with both the **Microsoft Store** and the **standalone installer** versions of Claude Desktop.

## Requirements

- Windows 10 or Windows 11
- [Claude Desktop](https://claude.ai/download) (Microsoft Store or installer build)
- Windows PowerShell 5.1 or later (pre-installed on Windows)

## Installation

1. Download or clone this repository:
   ```bash
   git clone https://github.com/HadiAghandeh/ClaudeMultiUser.git
   ```
2. That's it — no build step. Keep the files together in one folder.

## Usage

Double-click **`start.bat`**. You'll see a menu like this:

```text
  Claude Desktop - choose a user
  ------------------------------
   1)  work@company.com                 last used 2026-11-17 09:27
   2)  personal@example.com             never used

   M)  Main Claude Desktop (your normal profile)
   A)  Add a new user
   R)  Remove a user
   Q)  Quit
```

- **Add a user:** press `A`, enter an email or name, then sign in to Claude in the window that opens.
- **Start a user:** type its number and press Enter.
- **Remove a user:** press `R`, pick the number, and type `DELETE` to confirm. This deletes that profile's local data folder (your conversations remain on your Claude account).

## Command-line options

| Command | What it does |
| --- | --- |
| `start.bat` | Opens the interactive menu |
| `start.bat -Profile work@company.com` | Starts that account directly, no menu |
| `start.bat -List` | Prints all profiles and exits |

## Desktop shortcuts per account

Create a Windows shortcut for each Claude account so it opens in one click:

1. Right-click `start.bat` → **Create shortcut**.
2. Right-click the shortcut → **Properties**.
3. In **Target**, append the profile, e.g.:
   ```text
   "C:\path\to\start.bat" -Profile work@company.com
   ```
4. Optionally change the icon to Claude's and pin it to the taskbar or Start menu.

## How it works

Claude Desktop is an Electron app, and Electron supports the `--user-data-dir` switch. The launcher starts `Claude.exe` with a different data directory for each profile, which gives each one its own independent session:

```text
Claude.exe --user-data-dir="%LOCALAPPDATA%\Claude-Data\<profile>"
```

The script looks for Claude Desktop in these locations:

- `%LOCALAPPDATA%\Microsoft\WindowsApps\claude-desktop.exe` (Microsoft Store)
- `%LOCALAPPDATA%\Programs\Claude\Claude.exe`
- `%LOCALAPPDATA%\AnthropicClaude\Claude.exe`

If yours is elsewhere, add its path to `$ClaudeCandidates` at the top of `claude-profiles.ps1`.

## Where data is stored

| Item | Location |
| --- | --- |
| Profile data (login, settings, cache) | `%LOCALAPPDATA%\Claude-Data\<profile>` |
| Profile list | `profiles.json` next to the script |

`profiles.json` contains your account emails and local paths, so it is excluded from git via `.gitignore`.

## FAQ

**Can I run two Claude Desktop accounts at the same time?**
Yes. Each profile runs as its own Claude Desktop instance with its own window.

**Does this affect my normal Claude Desktop profile?**
No. Your main profile is untouched; open it from the menu with `M`.

**Will removing a profile delete my Claude conversations?**
No. It only deletes the local data folder on this PC. Conversations are stored on your Claude account.

**Does it work on macOS or Linux?**
This script is Windows-only. The same `--user-data-dir` approach works on other platforms with a small shell script.

**Is each profile's MCP / `claude_desktop_config.json` separate?**
Yes — every profile has its own data directory, so configuration is per profile.

## Troubleshooting

- **"Claude Desktop was not found"** — install Claude Desktop, or add your `Claude.exe` path to `$ClaudeCandidates`.
- **Script blocked by execution policy** — always launch through `start.bat`, which runs PowerShell with `-ExecutionPolicy Bypass` for this script only.
- **A new window opens in the wrong account** — close all Claude windows and launch the profile again from the menu.

## Disclaimer

This is an unofficial community tool and is not affiliated with or endorsed by Anthropic. "Claude" is a trademark of Anthropic, PBC.
