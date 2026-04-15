# Shell Environment Setup - "One Stop Shop"

> Cross-platform shell environment with Catppuccin Mocha theme and modern CLI tools

![Zsh](https://img.shields.io/badge/Zsh-Shell-89b4fa?style=for-the-badge&logo=gnu-bash)
![Tmux](https://img.shields.io/badge/Tmux-Terminal-a6e3a1?style=for-the-badge)
![Neovim](https://img.shields.io/badge/Neovim-vim.pack-cba6f7?style=for-the-badge&logo=neovim)
![Oh-My-Posh](https://img.shields.io/badge/Oh--My--Posh-Prompt-fab387?style=for-the-badge)
![Yazi](https://img.shields.io/badge/Yazi-File_Manager-f5c2e7?style=for-the-badge)
![Ghostty](https://img.shields.io/badge/Ghostty-Terminal-94e2d5?style=for-the-badge)
![Catppuccin](https://img.shields.io/badge/Theme-Catppuccin-1e1e2e?style=for-the-badge)

## Philosophy

- **Learn by configuring** - Every config is hand-written, not generated
- **Catppuccin Mocha everywhere** - Consistent theming across all tools
- **Modern replacements** - eza > ls, bat > cat, ripgrep > grep, fd > find, zoxide > cd
- **Cross-platform** - macOS (Intel + Apple Silicon) and Linux (Ubuntu, Arch, RHEL, Fedora)
- **Interactive TUI installer** - Go + Bubble Tea with dry-run, backup/restore, component selection

## Features

This setup provides a complete, modern shell environment with:

- **Zsh Shell** - Feature-rich shell with Antidote plugin manager (16 plugins)
- **Tmux** - Terminal multiplexer with session persistence and Catppuccin theme (11 plugins)
- **Neovim** - Modern editor with vim.pack built-in package manager (33 plugins, 14 LSP servers)
- **Oh-My-Posh** - Fast, customizable prompt with Git integration
- **Yazi** - Terminal file manager with image preview and Catppuccin theme (8 plugins)
- **Modern CLI Tools** - bat, ripgrep, fd, eza, fzf, zoxide, delta, lazygit, and more
- **Development Tools** - nvm + Node.js, uv (Python), Bun, Rust, .NET SDK
- **Shell Enhancements** - Atuin for shell history, fzf-tab for completions, direnv for per-project env
- **Catppuccin Mocha** - Beautiful, consistent theming across all tools

## Quick Installation

### One-Liner Installation

```bash
git clone https://github.com/chaseddevelopment/dotfiles ~/dotfiles && ~/dotfiles/install.sh
```

### Manual Installation

```bash
git clone https://github.com/chaseddevelopment/dotfiles ~/dotfiles
cd ~/dotfiles
chmod +x install.sh
./install.sh
```

## What Gets Installed

### Zsh Shell Configuration
- **Antidote Plugin Manager** - Fast, lightweight plugin management
- **zsh-autosuggestions** - Fish-like autosuggestions
- **zsh-syntax-highlighting** - Command syntax highlighting
- **fzf-tab** - Tab completions with fzf
- **zoxide Integration** - Smart directory navigation
- **Custom Aliases** - Modern tool replacements (eza, bat, rg, fd)
- **Modular Configuration** - Organized in `~/.config/zsh/`

### Tmux Configuration
- **TPM (Tmux Plugin Manager)** - Plugin management
- **Catppuccin Mocha Theme** - Beautiful color scheme
- **Vim-Tmux Navigator** - Seamless navigation between Vim and Tmux
- **Tmux Sensible** - Better default settings
- **Custom Key Bindings** - Ctrl+Space prefix and intuitive shortcuts

### Neovim Setup
- **vim.pack** - Neovim's built-in plugin manager (0.12+)
- **30+ plugins** - Catppuccin, Snacks (picker/dashboard), blink.cmp, treesitter, LSP, Mason, and more
- **Latest Version Installation**:
  - **macOS**: Uses `brew install --HEAD neovim` for latest features
  - **Arch Linux**: Installs `neovim-git` from AUR for version 0.12+
  - **Ubuntu/Debian**: Installs from GitHub releases for latest version
  - **Other platforms**: Uses system package manager with upgrade recommendations
- **LSP Integration** - Language servers for Python, TypeScript, Rust, Lua, C#, Bash, Docker, and more
- **Tree-sitter** - Syntax highlighting and code navigation for 30+ languages

### Oh-My-Posh Prompt
- **Catppuccin Theme** - Beautiful, informative prompt
- **Git Integration** - Branch, status, and commit information
- **Language Detection** - Automatic programming language indicators
- **Performance Optimized** - Fast prompt rendering

### Yazi File Manager
- **Terminal file manager** with image preview support
- **Catppuccin Mocha theme** - Consistent look
- **Plugins** - lazygit integration, git status, smart-enter, jump-to-char
- **Tmux passthrough** - Image previews work inside tmux

### Modern CLI Tools

| Modern Tool | Replaces | What It Does |
|-------------|----------|--------------|
| `eza` | `ls` | File listing with icons, colors, git status |
| `bat` | `cat` | Syntax highlighting, line numbers, git diff |
| `ripgrep` (`rg`) | `grep` | Lightning-fast recursive search |
| `fd` | `find` | User-friendly file finder |
| `zoxide` | `cd` | Smart directory navigation (learns your habits) |
| `fzf` | - | Fuzzy finder for files, history, everything |
| `delta` | `diff` | Syntax-highlighted git diffs with side-by-side view |
| `lazygit` | - | TUI git client for staging, branching, conflicts |
| `xh` | `curl` | Modern HTTP client with JSON highlighting |
| `tailspin` (`tspin`) | `tail` | Pretty log viewer with auto-highlighting |
| `jq` / `yq` | - | JSON and YAML processing on the command line |

### Development Tools
- **nvm** - Node.js version manager (with LTS installed)
- **uv** - Fast Python package installer and environment manager
- **Bun** - Fast JavaScript runtime
- **Rust** - Systems programming language and cargo package manager
- **ruff** - Fast Python linter and formatter
- **.NET SDK** - For F#/C# LSP support
- **tree-sitter CLI** - Parser generator for syntax highlighting

### Ghostty Terminal (Desktop Only)
- **Catppuccin Mocha Theme** - Consistent with all other tools
- **JetBrainsMono Nerd Font** - Ligatures and icons
- **Semi-transparent Background** - With wallpaper support
- **SSH Integration** - Warp-like prompt navigation

### Shell Enhancements
- **Atuin** - Magical shell history with sync and search
- **direnv** - Auto-load environment variables per project directory
- **Git Config** - Delta pager, Catppuccin-themed diffs

## System Requirements

### Supported Operating Systems
- **macOS** (Intel and Apple Silicon)
- **Ubuntu/Debian** (18.04+)
- **RHEL/CentOS/Fedora** (7+)
- **Arch Linux**
- **Other Linux distributions** (with manual package installation)

### Prerequisites
- **Git** (version 2.0+)
- **curl** or **wget**
- **Internet connection** for downloading the installer and packages

### Optional but Recommended
- **A Nerd Font** for proper icon display ([Download here](https://www.nerdfonts.com/))
- **Terminal with true color support** (most modern terminals)

## How It Works

`./install.sh` is a bootstrap script that:
1. Downloads the `dotsetup` binary from GitHub Releases (auto-detects OS and architecture)
2. Checks for updates on subsequent runs
3. Launches the interactive TUI installer

All options (dry-run, component selection, backup/restore, update) are available
in the TUI — no CLI flags needed.

### Build from Source

If you prefer to build the installer yourself (requires Go 1.22+):
```bash
cd installer && go build -o dotsetup . && cd .. && ./install.sh
```

## Key Bindings

### Zsh Shell
| Key Combination | Action |
|-----------------|--------|
| `Ctrl+T` | File search with FZF |
| `Ctrl+R` | History search with Atuin |
| `Alt+C` | Directory search with FZF |
| `Tab` | Autocompletion with fzf-tab |
| `->` | Accept autosuggestion |
| `Ctrl+X Ctrl+E` | Edit command in $EDITOR |

### Tmux
| Key Combination | Action |
|-----------------|--------|
| `Ctrl+Space` | Prefix key |
| `Prefix + \|` | Split horizontally |
| `Prefix + -` | Split vertically |
| `Prefix + c` | New window |
| `Alt+H` | Previous window |
| `Alt+L` | Next window |
| `Ctrl+H/J/K/L` | Navigate panes (with vim-tmux-navigator) |
| `Prefix + I` | Install TPM plugins |
| `Prefix + U` | Update TPM plugins |
| `Prefix + r` | Reload configuration |

### Neovim
| Key Combination | Action |
|-----------------|--------|
| `<Space>` | Leader key |
| `<Leader>e` | File explorer (Snacks) |
| `<Leader>ff` | Find files |
| `<Leader>fg` | Live grep |
| `<Leader>fb` | Buffers |
| `<Leader>fr` | Recent files |
| `<Leader>o` | Code outline (Aerial) |
| `<Leader>ha` | Harpoon add file |
| `<Leader>hh` | Harpoon menu |
| `<Leader>gd` | Diff view |
| `<Leader>ac` | Toggle Claude Code |
| `<Leader>xx` | Diagnostics (Trouble) |
| `:lua vim.pack.update()` | Update all plugins |

### Yazi
| Key Combination | Action |
|-----------------|--------|
| `g i` | Open lazygit |
| `Enter` | Smart enter (open file or dir) |
| `f` | Jump to char |

## Customization

### Adding Zsh Aliases
Edit `~/.config/zsh/aliases/general.zsh`:
```zsh
alias myalias="my command"
```

### Adding Zsh Functions
Create a file in `~/.config/zsh/functions/`:
```zsh
my_function() {
    # Your function code here
}
```

### Adding Per-Project Environment Variables (direnv)
Create a `.envrc` file in any project directory:
```bash
# direnv will auto-load this when you cd into the directory
export MY_VAR="value"
layout python    # auto-activate Python venv
```
Then run `direnv allow` to trust the file.

### Installing Additional Tmux Plugins
Edit `~/.config/tmux/tmux.conf` and add:
```bash
set -g @plugin 'plugin-name'
```
Then press `Prefix + I` to install.

### Using Modern CLI Tools
The setup replaces traditional commands with modern alternatives:
```bash
# These commands now use modern tools automatically:
ls      # -> eza (with icons and colors)
cat     # -> bat (with syntax highlighting)
find    # -> fd (faster and user-friendly)
grep    # -> ripgrep (much faster)
cd      # -> zoxide (learns your habits)

# Original commands are still available as:
command ls    # Use original ls
command cat   # Use original cat
```

### Privileged Editing
Use `snvim` to edit files that require root access. It uses `sudoedit` which runs
Neovim as your user (with full config and plugins) and writes back as root:
```bash
snvim /etc/hosts
```

## Troubleshooting

### Common Issues

#### Zsh Shell Not Set as Default
```bash
# Check available shells
cat /etc/shells

# Add Zsh to shells if missing
echo $(which zsh) | sudo tee -a /etc/shells

# Set as default shell
chsh -s $(which zsh)
```

#### Tmux Plugins Not Working
```bash
# Manually install TPM
git clone https://github.com/tmux-plugins/tpm ~/.tmux/plugins/tpm

# Install plugins
~/.tmux/plugins/tpm/scripts/install_plugins.sh
```

#### Neovim Plugins Not Installing
```bash
# Clear Neovim cache
rm -rf ~/.local/share/nvim
rm -rf ~/.local/state/nvim
rm -rf ~/.cache/nvim

# Restart Neovim - plugins will auto-install
nvim
```

#### Oh-My-Posh Not Showing
```bash
# Check if Oh-My-Posh is in PATH
which oh-my-posh

# Verify it's in .zshrc (should be automatic)
grep oh-my-posh ~/.config/zsh/.zshrc
```

#### Icons Not Displaying
1. Install a Nerd Font from [nerdfonts.com](https://www.nerdfonts.com/)
2. Set your terminal to use the Nerd Font
3. Restart your terminal

### Log Files
- Backup location: `~/.dotfiles-backup-[timestamp]`

### Getting Help
1. Use the dry-run option in the TUI to preview changes
2. Create an issue in this repository with:
   - Your operating system and version
   - Terminal emulator being used
   - Full error message

## Updating

### Update Everything
```bash
cd ~/dotfiles
git pull
./install.sh
```

Run the installer and select the update option from the TUI menu.

### Update Zsh Plugins
Plugins are managed by Antidote and update automatically, or run:
```bash
antidote update
```

### Update Tmux Plugins
```bash
# In tmux session: Prefix + U
```

### Update Neovim Plugins
```vim
:lua vim.pack.update()
```

## File Structure

```
dotfiles/
├── README.md                    # This file
├── install.sh                   # Bootstrap script (downloads + runs dotsetup)
├── installer/                   # Go TUI installer (Bubble Tea + Lipgloss)
│   ├── main.go                 # Entry point
│   ├── go.mod / go.sum         # Go dependencies
│   └── internal/               # TUI, registry, config, executor, etc.
├── configs/
│   ├── zsh/
│   │   ├── .zshenv             # Environment variables
│   │   ├── .zshrc              # Main Zsh configuration
│   │   ├── aliases/            # Alias definitions (general, git)
│   │   ├── functions/          # Custom functions (ansible, apt, ssh, utils)
│   │   ├── plugins/            # Antidote plugin manifest
│   │   └── tools/              # Tool-specific configs (nvm, bun, tmux-auto, yazi)
│   ├── nvim/
│   │   ├── init.lua            # Neovim entry point (vim.pack)
│   │   ├── lsp/                # LSP server configurations
│   │   └── lua/                # Plugin and core configuration
│   ├── tmux/
│   │   ├── tmux.conf           # Tmux configuration
│   │   └── catppuccin-modules/ # Custom Catppuccin status modules
│   ├── oh-my-posh/
│   │   └── config.omp.yaml     # Oh-My-Posh configuration
│   ├── atuin/
│   │   └── config.toml         # Atuin configuration
│   ├── ghostty/
│   │   └── config              # Ghostty terminal configuration
│   ├── git/
│   │   └── config              # Git config (delta pager, merge settings)
│   ├── lazygit/
│   │   └── config.yml          # Lazygit Catppuccin theme
│   └── yazi/
│       ├── yazi.toml           # Yazi configuration
│       ├── keymap.toml         # Yazi keybindings
│       ├── theme.toml          # Yazi theme
│       ├── init.lua            # Yazi init (plugin loading)
│       ├── package.toml        # Yazi plugin manifest
│       ├── plugins/            # Yazi plugins (lazygit, git, etc.)
│       └── flavors/            # Catppuccin Mocha flavor
```

## Contributing

1. Fork this repository
2. Create a feature branch: `git checkout -b feature-name`
3. Make your changes and test them
4. Commit your changes: `git commit -am 'Add feature'`
5. Push to the branch: `git push origin feature-name`
6. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Zsh](https://www.zsh.org/) - Powerful shell
- [Antidote](https://github.com/mattmc3/antidote) - Fast Zsh plugin manager
- [Tmux](https://github.com/tmux/tmux) - Terminal multiplexer
- [Neovim](https://neovim.io/) - Hyperextensible Vim-based text editor
- [Oh-My-Posh](https://ohmyposh.dev/) - Cross-shell prompt
- [Yazi](https://yazi-rs.github.io/) - Terminal file manager
- [Catppuccin](https://github.com/catppuccin/catppuccin) - Beautiful color schemes
- [Atuin](https://github.com/atuinsh/atuin) - Magical shell history
- [delta](https://github.com/dandavison/delta) - Syntax-highlighting pager for git
- [lazygit](https://github.com/jesseduffield/lazygit) - TUI git client
- [TPM](https://github.com/tmux-plugins/tpm) - Tmux plugin manager

---

**Made with care for developers who love beautiful, functional terminals**
