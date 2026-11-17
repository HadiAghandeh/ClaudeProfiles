<img src="docs/icon.png" width="96" align="right" alt="Claude Profiles app icon: two overlapping profile silhouettes">

# Claude Desktop & Claude Code Multiple Accounts on Windows — Profile Switcher

**Run multiple Claude Desktop and Claude Code accounts side by side on one Windows PC.** This lightweight launcher — available as a single-file Go `.exe` or a PowerShell script — gives every Claude user its own isolated profile — separate login, settings, MCP config and data — for both the Claude Desktop app and the Claude Code CLI, so you can switch between work and personal Claude accounts (or several team accounts) without signing out.

[![Release](https://img.shields.io/github/v/release/HadiAghandeh/ClaudeMultiUser)](https://github.com/HadiAghandeh/ClaudeMultiUser/releases/latest)
[![CI](https://github.com/HadiAghandeh/ClaudeMultiUser/actions/workflows/ci.yml/badge.svg)](https://github.com/HadiAghandeh/ClaudeMultiUser/actions/workflows/ci.yml)

<p align="center">
  <img src="docs/screenshot.png" width="760" alt="Claude Profiles window on Windows listing four Claude accounts, each with buttons to open Claude Desktop or Claude Code as that account">
</p>

> Keywords: Claude Desktop multiple accounts, Claude Code multiple accounts, Claude Code switch account, `CLAUDE_CONFIG_DIR`, Claude multi-user, Claude profile manager, switch Claude accounts, run two Claude Desktop instances, Anthropic Claude Windows, `--user-data-dir`, Claude shortcut per account, Claude Start menu shortcut, PowerShell launcher, Go CLI.

---

## Table of contents

- [Why use this](#why-use-this)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Go version (.exe)](#go-version-exe)
- [Usage](#usage)
- [Command-line options](#command-line-options)
- [Claude Code profiles](#claude-code-profiles)
- [Desktop shortcuts per account](#desktop-shortcuts-per-account)
- [How it works](#how-it-works)
- [Where data is stored](#where-data-is-stored)
- [FAQ](#faq)
- [Troubleshooting](#troubleshooting)
- [Disclaimer](#disclaimer)

## Why use this

Claude Desktop signs in to one account at a time. If you have a personal Claude Pro account and a company Claude Team or Enterprise account, you normally have to log out and back in every time you switch. This tool launches each account in its **own Claude Desktop window with its own user data folder**, so all your accounts stay signed in and can run at the same time. The same goes for **Claude Code**: each profile opens a terminal with its own Claude Code config folder and login, so `claude` runs as the right account.

## Features

- 🧑‍🤝‍🧑 **Multiple Claude accounts at once** — each profile opens in its own window.
- 🖼️ **Desktop app with a real window** — `claude-profiles-gui.exe` lists your profiles with one-click **Claude Desktop** and **Claude Code** buttons, a folder picker, and light/dark mode.
- 🚀 **Desktop and Start menu shortcuts per account** — one click creates a shortcut like `Claude - work@company.com` or `Claude Code - work@company.com`, so each account opens straight from the Desktop, the Start menu or Windows search, and can be pinned to the taskbar. Removing a profile removes its shortcuts. [See how ↓](#desktop-shortcuts-per-account)
- 💻 **Claude Code profile switcher** — start the `claude` CLI as any profile in a new terminal (Windows Terminal when available).
- 🔒 **Fully isolated profiles** — separate login session, cookies, settings, MCP servers and cache.
- 🟢 **See what's open** — profiles with Claude Desktop currently running are marked **Open**, updated live in the window (and with `*` in the terminal menu).
- ✏️ **Rename profiles** — change a profile's email or name without signing out; its data stays put and its shortcuts are renamed to match.
- 📋 **Simple interactive menu** — pick a user by number, add (`A`), rename (`N`), remove (`R`), create shortcuts (`S`), or open your main profile (`M`).
- 🔍 **Auto-discovery** — existing profile folders on disk are picked up automatically.
- 🕒 **Last-used tracking** — the menu shows when each profile was last opened.
- 🪶 **No install, no dependencies** — download one `.exe`, or use the PowerShell script that ships with Windows.
- 📦 **Prebuilt releases** — Windows x64 and ARM64 binaries on [GitHub Releases](https://github.com/HadiAghandeh/ClaudeMultiUser/releases).
- 🏪 Works with both the **Microsoft Store** and the **standalone installer** versions of Claude Desktop.

## Requirements

- Windows 10 or Windows 11
- [Claude Desktop](https://claude.ai/download) (Microsoft Store or installer build)
- Windows PowerShell 5.1 or later (pre-installed on Windows)
- Optional: [Claude Code](https://docs.claude.com/en/docs/claude-code/setup) for the Claude Code profiles
- Optional: Windows Terminal (used for Claude Code tabs when installed)
- For the windowed app: Microsoft Edge WebView2 Runtime (already included in Windows 11 and up-to-date Windows 10)

## Installation

**Option A — Go executable (recommended):**

1. Download `claude-profiles_<version>_windows_amd64.zip` (or `_arm64` for ARM PCs) from the [latest release](https://github.com/HadiAghandeh/ClaudeMultiUser/releases/latest).
2. Unzip it to a folder of your choice.
3. Double-click **`claude-profiles-gui.exe`** for the windowed app, or run `claude-profiles.exe` for the terminal menu.

**Option B — PowerShell script:**

1. Download or clone this repository:
   ```bash
   git clone https://github.com/HadiAghandeh/ClaudeMultiUser.git
   ```
2. Double-click `start.bat`. No build step — keep the files together in one folder.

## Go version (.exe)

The release zip contains two programs built from the same Go code:

- **`claude-profiles-gui.exe`** — a desktop window. Add a profile by email, then click **Claude Desktop** or **Claude Code** next to it. Claude Code asks for a project folder (type it or click **Browse…**). Profiles with Claude Desktop running show a green **Open** marker. Hover a name and click the pencil to rename it (Enter saves, Esc cancels). The shortcut icon creates Desktop and Start menu shortcuts for a profile, the trash icon removes a profile after confirmation, and **Open main Claude Desktop** starts your normal profile. It follows the Windows light/dark theme. Started with `-profile` it launches directly without opening the window, which is handy for shortcuts.
- **`claude-profiles.exe`** — the terminal version below.

`claude-profiles.exe` has the same menu and features as the PowerShell script, in a single file with no dependencies. Both use the same `profiles.json` format and data folders, so you can switch between them (put the `.exe` next to `start.bat` to share one profile list).

| Command | What it does |
| --- | --- |
| `claude-profiles.exe` | Opens the interactive menu |
| `claude-profiles.exe -profile work@company.com` | Starts that account's Claude Desktop |
| `claude-profiles.exe -profile work@company.com -code` | Starts that account's Claude Code in the current folder |
| `claude-profiles.exe -profile work@company.com -code -dir C:\src\app` | Starts that account's Claude Code in `C:\src\app` |
| `claude-profiles.exe -list` | Prints all profiles, and whether Claude Desktop is running for each, and exits |
| `claude-profiles.exe -profile work@company.com -rename new@company.com` | Renames that profile (it stays signed in; shortcuts are updated) |
| `claude-profiles.exe -profile work@company.com -shortcut both` | Creates Desktop and Start menu shortcuts for that account (`desktop`, `start` or `both`; add `-code` for Claude Code) |
| `claude-profiles.exe -profiles-file D:\profiles.json` | Uses a different profile list |
| `claude-profiles.exe -version` | Prints the version |

**Build from source** (Go 1.22+):

```bash
go build -o claude-profiles.exe .
```

```bash
go build -tags gui -ldflags "-H=windowsgui" -o claude-profiles-gui.exe .
```

The window is plain HTML in [`ui/index.html`](ui/index.html), embedded into the `.exe` and shown with [WebView2](https://github.com/jchv/go-webview2) (no CGO, so it cross-compiles).

**App icon:** drawn by [`tools/icongen`](tools/icongen/main.go) into [`winres/icon.ico`](winres/icon.ico), and embedded (with version details) through the `rsrc_windows_*.syso` files, which Go links in automatically. After changing the icon or [`winres/winres.json`](winres/winres.json), regenerate them:

```bash
go run ./tools/icongen
```

```bash
go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --arch amd64,arm64 --product-version 0.0.0 --file-version 0.0.0
```

The committed files say version `0.0.0` (a development build); releases rerun the second step with the release's version number.

**Publishing a release:** releases are built by [GoReleaser](https://goreleaser.com) in GitHub Actions. Push a version tag and the workflow attaches the zips and checksums to a new GitHub release:

```bash
git tag v1.0.0
```

```bash
git push origin v1.0.0
```

## Usage

Double-click **`start.bat`**. You'll see a menu like this:

```text
  Claude - choose a user
  ----------------------
   1)  work@company.com                 last used 2026-11-17 09:27
   2)  personal@example.com             never used

   1, 2, ...   Claude Desktop as that user
  C1, C2, ...  Claude Code as that user (opens a terminal)

   M)  Main Claude Desktop (your normal profile)
   A)  Add a new user
   R)  Remove a user
   Q)  Quit
```

- **Add a user:** press `A`, enter an email or name, then choose Desktop or Code and sign in.
- **Start Claude Desktop as a user:** type its number (e.g. `1`) and press Enter.
- **Start Claude Code as a user:** type `C` plus its number (e.g. `C1`), then the project folder to open.
- **Remove a user:** press `R`, pick the number, and type `DELETE` to confirm. This deletes that profile's local data folder (your conversations remain on your Claude account).

## Command-line options

For the PowerShell script (`start.bat`):


| Command | What it does |
| --- | --- |
| `start.bat` | Opens the interactive menu |
| `start.bat -Profile work@company.com` | Starts that account's Claude Desktop directly, no menu |
| `start.bat -Profile work@company.com -Code` | Starts that account's Claude Code in the current folder |
| `start.bat -Profile work@company.com -Code -Dir C:\src\app` | Starts that account's Claude Code in `C:\src\app` |
| `start.bat -List` | Prints all profiles and exits |

## Claude Code profiles

Every profile works for Claude Code too. Picking `C1` (or running `start.bat -Profile <name> -Code`) opens a new terminal where:

- `CLAUDE_CONFIG_DIR` points to `%LOCALAPPDATA%\Claude-Code-Data\<profile>`, so login, `settings.json`, MCP servers, memory and history are separate per account;
- the terminal starts in the project folder you chose, and `claude` is launched for you;
- the window title shows which account it is, e.g. `Claude Code - work@company.com`.

The first time you open Claude Code for a profile, run `/login` and sign in with that account. Claude Desktop and Claude Code logins are independent, so sign in to each once.

> Tip: to use your existing Claude Code setup as a starting point, copy files such as `settings.json` from `%USERPROFILE%\.claude` into the profile's `Claude-Code-Data` folder.

## Desktop shortcuts per account

Give each Claude account its own one-click shortcut on the Desktop and in the Start menu.

**With the app (easiest):** in `claude-profiles-gui.exe`, click the shortcut icon next to a profile, choose **Claude Desktop** or **Claude Code** (with an optional project folder), and tick **Desktop**, **Start menu**, or both. Shortcuts are named like `Claude - work@company.com` and `Claude Code - work@company.com`. Start menu shortcuts go in a **Claude Profiles** folder, so you can also pin them to Start or the taskbar. Removing a profile removes its shortcuts too.

<p align="center">
  <img src="docs/start-menu-shortcut.webp" width="640" alt="Windows 11 Start menu showing a 'Claude - a@second.com' shortcut with the Claude Profiles icon under Recently added, opening Claude Desktop as that account">
</p>

Each account gets its own entry — search the Start menu for its email (e.g. `Claude - a@second.com`) to open Claude as that account in one click.

From the terminal version, press `S` in the menu, or:

```bash
claude-profiles.exe -profile work@company.com -shortcut both
```

```bash
claude-profiles.exe -profile work@company.com -code -dir "C:\src\app" -shortcut desktop
```

Shortcuts run `claude-profiles-gui.exe`, so no console window flashes. Keep the `.exe` files where they are after creating shortcuts, or create them again after moving the folder.

**By hand, for the PowerShell script:**

1. Right-click `start.bat` → **Create shortcut**.
2. Right-click the shortcut → **Properties**.
3. In **Target**, append the profile, e.g.:
   ```text
   "C:\path\to\start.bat" -Profile work@company.com
   ```
4. Optionally change the icon to Claude's and pin it to the taskbar or Start menu.

For a Claude Code shortcut, add `-Code -Dir "C:\path\to\project"` to the target.

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

For Claude Code, the launcher sets the documented `CLAUDE_CONFIG_DIR` environment variable, which tells Claude Code where to keep its config and credentials instead of `~/.claude`:

```powershell
$env:CLAUDE_CONFIG_DIR = "$env:LOCALAPPDATA\Claude-Code-Data\<profile>"
claude
```

It finds `claude` on your `PATH`, or in `%USERPROFILE%\.local\bin` or `%APPDATA%\npm`; add other paths to `$ClaudeCodeCandidates`.

## Where data is stored

| Item | Location |
| --- | --- |
| Claude Desktop data (login, settings, cache) | `%LOCALAPPDATA%\Claude-Data\<profile>` |
| Claude Code config (login, settings, MCP, history) | `%LOCALAPPDATA%\Claude-Code-Data\<profile>` |
| Profile list | `profiles.json` next to the script or `.exe` |

`profiles.json` contains your account emails and local paths, so it is excluded from git via `.gitignore`.

## FAQ

**Can I run two Claude Desktop accounts at the same time?**
Yes. Each profile runs as its own Claude Desktop instance with its own window.

**Does this affect my normal Claude Desktop profile?**
No. Your main profile is untouched; open it from the menu with `M`.

**Will removing a profile delete my Claude conversations?**
No. It only deletes the local data folder on this PC. Conversations are stored on your Claude account.

**Can I use multiple Claude Code accounts at the same time?**
Yes. Each Claude Code profile runs in its own terminal with its own `CLAUDE_CONFIG_DIR`, so two accounts can work in parallel.

**Does the Claude Code profile change my normal `~/.claude` setup?**
No. Running `claude` in a normal terminal still uses `%USERPROFILE%\.claude`.

**Does it work on macOS or Linux?**
This script is Windows-only. The same `--user-data-dir` and `CLAUDE_CONFIG_DIR` approach works on other platforms with a small shell script.

**Is each profile's MCP / `claude_desktop_config.json` separate?**
Yes — every profile has its own data directory, so configuration is per profile.

## Troubleshooting

- **"Claude Desktop was not found"** — install Claude Desktop, or add your `Claude.exe` path to `$ClaudeCandidates`.
- **"Claude Code was not found"** — install Claude Code, or add your `claude.exe` path to `$ClaudeCodeCandidates`.
- **Claude Code asks me to log in again** — expected the first time for each profile; run `/login` once per profile.
- **Script blocked by execution policy** — always launch through `start.bat`, which runs PowerShell with `-ExecutionPolicy Bypass` for this script only.
- **A new window opens in the wrong account** — close all Claude windows and launch the profile again from the menu.

## Disclaimer

This is an unofficial community tool and is not affiliated with or endorsed by Anthropic. "Claude" is a trademark of Anthropic, PBC.
