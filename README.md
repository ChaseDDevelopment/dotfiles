# Shell Environment Setup - "One Stop Shop"

> Cross-platform shell environment with Catppuccin Mocha theme and modern CLI tools

![Zsh](https://img.shields.io/badge/Zsh-Shell-89b4fa?style=for-the-badge&logo=gnu-bash)
![Tmux](https://img.shields.io/badge/Tmux-Terminal-a6e3a1?style=for-the-badge)
![Neovim](https://img.shields.io/badge/Neovim-LazyVim-cba6f7?style=for-the-badge&logo=neovim)
![Starship](https://img.shields.io/badge/Starship-Prompt-fab387?style=for-the-badge)
![Catppuccin](https://img.shields.io/badge/Theme-Catppuccin-1e1e2e?style=for-the-badge)

## Features

This setup provides a complete, modern shell environment with:

- **Zsh Shell** - Feature-rich shell with Antidote plugin manager
- **Tmux** - Terminal multiplexer with session management and Catppuccin theme
- **Neovim** - Modern Vim-based editor with LazyVim configuration
- **Starship** - Fast, customizable prompt with Git integration
- **Modern CLI Tools** - bat, ripgrep, fd, eza, fzf, zoxide, and more
- **Development Tools** - nvm + Node.js, uv (Python), Bun
- **Shell Enhancements** - Atuin for shell history, fzf-tab for completions
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
- **LazyVim Configuration** - Modern Neovim setup
- **Latest Version Installation**:
  - **macOS**: Uses `brew install --HEAD neovim` for latest features
  - **Arch Linux**: Installs `neovim-git` from AUR for version 0.12+
  - **Other platforms**: Uses system package manager with upgrade recommendations
- **Plugin Management** - Automatic plugin installation on first startup
- **LSP Integration** - Language server support for multiple languages

### Starship Prompt
- **Catppuccin Theme** - Beautiful, informative prompt
- **Git Integration** - Branch, status, and commit information
- **Language Detection** - Automatic programming language indicators
- **Performance Optimized** - Fast prompt rendering

### Modern CLI Tools
- **bat** - Better cat with syntax highlighting and Git integration
- **ripgrep (rg)** - Lightning-fast grep replacement
- **fd** - User-friendly find replacement
- **eza** - Modern ls replacement with icons and colors
- **fzf** - Command-line fuzzy finder
- **zoxide** - Smart cd that learns your habits

### Development Tools
- **nvm** - Node.js version manager (with LTS installed)
- **uv** - Fast Python package installer and environment manager
- **Bun** - Fast JavaScript runtime

### Shell Enhancements
- **Atuin** - Magical shell history with sync and search
- **Ghostty Config** - Terminal emulator configuration (desktop only)

## System Requirements

### Supported Operating Systems
- **macOS** (Intel and Apple Silicon)
- **Ubuntu/Debian** (18.04+)
- **RHEL/CentOS/Fedora** (7+)
- **Arch Linux**
- **Other Linux distributions** (with manual package installation)

### Prerequisites
- **Git** (version 2.0+)
- **curl**
- **Bash 4.2+** (installed automatically on macOS via Homebrew)
- **Internet connection** for downloading packages and configurations

### Optional but Recommended
- **A Nerd Font** for proper icon display ([Download here](https://www.nerdfonts.com/))
- **Terminal with true color support** (most modern terminals)

## Installation Options

### Standard Installation
```bash
./install.sh
```

### Dry Run (Preview Only)
```bash
./install.sh --dry-run
```

### Skip Package Installation
```bash
./install.sh --skip-packages
```

### Configuration Only
```bash
./install.sh --config-only
```

### Restore from Backup
```bash
./install.sh --restore-backup ~/.dotfiles-backup-20240101-120000
```

### Verbose Output
```bash
./install.sh --verbose
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
| `Prefix + r` | Reload configuration |

### Neovim (LazyVim)
| Key Combination | Action |
|-----------------|--------|
| `<Space>` | Leader key |
| `<Leader>e` | File explorer |
| `<Leader>ff` | Find files |
| `<Leader>fg` | Live grep |
| `<Leader>gg` | LazyGit |
| `:Lazy` | Plugin manager |

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

### Installing Additional Tmux Plugins
Edit `~/.tmux.conf` and add:
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

#### Starship Not Showing
```bash
# Check if Starship is in PATH
which starship

# Verify it's in .zshrc (should be automatic)
grep starship ~/.config/zsh/.zshrc
```

#### Icons Not Displaying
1. Install a Nerd Font from [nerdfonts.com](https://www.nerdfonts.com/)
2. Set your terminal to use the Nerd Font
3. Restart your terminal

### Log Files
- Installation log: `~/dotfiles/install.log`
- Backup location: `~/.dotfiles-backup-[timestamp]`

### Getting Help
1. Check the installation log for errors
2. Run the installer with `--dry-run` to see what would be executed
3. Run with `--verbose` for detailed output
4. Create an issue in this repository with:
   - Your operating system and version
   - Terminal emulator being used
   - Full error message
   - Installation log contents

## Updating

### Update All Configurations
```bash
cd ~/dotfiles
git pull
./install.sh --config-only
```

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
```bash
# In Neovim
:Lazy update
```

## File Structure

```
dotfiles/
├── README.md                    # This file
├── install.sh                   # Main installer script
├── scripts/
│   ├── detect-os.sh            # OS detection utilities
│   ├── install-packages.sh     # Package installation
│   ├── install-tools.sh        # Official source tool installers
│   ├── setup-zsh.sh            # Zsh shell setup
│   ├── setup-tmux.sh           # Tmux setup
│   ├── setup-neovim.sh         # Neovim setup
│   ├── setup-starship.sh       # Starship setup
│   ├── setup-atuin.sh          # Atuin setup
│   └── setup-ghostty.sh        # Ghostty setup (desktop only)
├── configs/
│   ├── zsh/
│   │   ├── .zshenv             # Environment variables
│   │   ├── .zshrc              # Main Zsh configuration
│   │   ├── aliases/            # Alias definitions
│   │   ├── functions/          # Custom functions
│   │   ├── plugins/            # Antidote plugin manifest
│   │   └── tools/              # Tool-specific configs (nvm, bun)
│   ├── nvim/
│   │   ├── init.lua            # Neovim entry point
│   │   └── lua/                # LazyVim configuration
│   ├── tmux/
│   │   └── .tmux.conf          # Tmux configuration
│   ├── starship/
│   │   └── starship.toml       # Starship configuration
│   ├── atuin/
│   │   └── config.toml         # Atuin configuration
│   └── ghostty/
│       └── config              # Ghostty terminal configuration
└── backups/                     # Backup directory
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
- [LazyVim](https://www.lazyvim.org/) - Neovim configuration framework
- [Starship](https://starship.rs/) - Cross-shell prompt
- [Catppuccin](https://github.com/catppuccin/catppuccin) - Beautiful color schemes
- [Atuin](https://github.com/atuinsh/atuin) - Magical shell history
- [TPM](https://github.com/tmux-plugins/tpm) - Tmux plugin manager

---

**Made with care for developers who love beautiful, functional terminals**
