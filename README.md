# 🐟 Shell Environment Setup - "One Stop Shop"

> Complete shell environment setup for Fish, Tmux, Neovim, and Starship with a single command

![Fish Shell](https://img.shields.io/badge/Fish-Shell-blue?style=for-the-badge&logo=gnu-bash)
![Tmux](https://img.shields.io/badge/Tmux-Terminal-green?style=for-the-badge)
![Neovim](https://img.shields.io/badge/Neovim-LazyVim-purple?style=for-the-badge&logo=neovim)
![Starship](https://img.shields.io/badge/Starship-Prompt-orange?style=for-the-badge)

## ✨ Features

This setup provides a complete, modern shell environment with:

- **🐟 Fish Shell** - Modern shell with intelligent autocompletion and syntax highlighting
- **🔧 Tmux** - Terminal multiplexer with session management and beautiful themes
- **⚡ Neovim** - Modern Vim-based editor with LazyVim configuration
- **🚀 Starship** - Fast, customizable prompt with Git integration
- **📦 Modern Tools** - eza, fzf, bun, nvm, and more
- **🎨 Catppuccin Theme** - Beautiful, consistent theming across all tools

## 🚀 Quick Installation

### One-Liner Installation

```bash
curl -sL https://raw.githubusercontent.com/[your-username]/dotfiles/main/install.sh | bash
```

### Manual Installation

```bash
git clone https://github.com/[your-username]/dotfiles.git ~/dotfiles
cd ~/dotfiles
chmod +x install.sh
./install.sh
```

## 📋 What Gets Installed

### 🐟 Fish Shell Configuration
- **Fisher Plugin Manager** - Plugin management system
- **NVM for Fish** - Node.js version management
- **FZF Integration** - Fuzzy finding for files, history, and directories
- **Custom Abbreviations** - 50+ useful command shortcuts
- **Tmux Session Management** - Automatic session creation and cleanup
- **Path Management** - Intelligent PATH configuration for all tools

### 🔧 Tmux Configuration
- **TPM (Tmux Plugin Manager)** - Plugin management
- **Catppuccin Theme** - Beautiful Mocha color scheme
- **Vim-Tmux Navigator** - Seamless navigation between Vim and Tmux
- **Tmux Sensible** - Better default settings
- **Tmux Yank** - System clipboard integration
- **Custom Key Bindings** - Ctrl+Space prefix and intuitive shortcuts

### ⚡ Neovim Setup
- **LazyVim Configuration** - Modern Neovim setup from [ChaseDDevelopment/neovim](https://github.com/ChaseDDevelopment/neovim)
- **Plugin Management** - Automatic plugin installation on first startup
- **LSP Integration** - Language server support for multiple languages
- **Modern UI** - Beautiful interface with file explorer and status line

### 🚀 Starship Prompt
- **Catppuccin Powerline Theme** - Beautiful, informative prompt
- **Git Integration** - Branch, status, and commit information
- **Language Detection** - Automatic programming language indicators
- **Performance Optimized** - Fast prompt rendering

### 📦 Additional Tools
- **eza** - Modern replacement for ls with icons and colors
- **fzf** - Command-line fuzzy finder
- **bun** - Fast JavaScript runtime and package manager
- **nvm** - Node.js version manager
- **Starship** - Cross-shell prompt

## 🖥️ System Requirements

### Supported Operating Systems
- **macOS** (Intel and Apple Silicon)
- **Ubuntu/Debian** (18.04+)
- **RHEL/CentOS/Fedora** (7+)
- **Arch Linux**
- **Other Linux distributions** (with manual package installation)

### Prerequisites
- **Git** (version 2.0+)
- **curl** or **wget**
- **Internet connection** for downloading packages and configurations

### Optional but Recommended
- **A Nerd Font** for proper icon display ([Download here](https://www.nerdfonts.com/))
- **Terminal with true color support** (most modern terminals)

## 🛠️ Installation Options

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

## ⌨️ Key Bindings

### Fish Shell
| Key Combination | Action |
|-----------------|--------|
| `Ctrl+T` | File search with FZF |
| `Ctrl+R` | History search with FZF |
| `Alt+C` | Directory search with FZF |
| `Tab` | Autocompletion |
| `→` | Accept autosuggestion |

### Tmux
| Key Combination | Action |
|-----------------|--------|
| `Ctrl+Space` | Prefix key |
| `Prefix + "` | Split horizontally |
| `Prefix + %` | Split vertically |
| `Prefix + c` | New window |
| `Alt+H` | Previous window |
| `Alt+L` | Next window |
| `Ctrl+H/J/K/L` | Navigate panes (with vim-tmux-navigator) |

### Neovim (LazyVim)
| Key Combination | Action |
|-----------------|--------|
| `<Space>` | Leader key |
| `<Leader>e` | File explorer |
| `<Leader>ff` | Find files |
| `<Leader>fg` | Live grep |
| `<Leader>gg` | LazyGit |
| `:Lazy` | Plugin manager |

## 🎨 Customization

### Changing Starship Theme
```bash
# List available presets
starship preset

# Apply a different preset
starship preset minimal -o ~/.config/starship.toml
```

### Adding Fish Abbreviations
Edit `~/.config/fish/conf.d/abbr.fish`:
```fish
abbr myabbr "my command"
```

### Installing Additional Tmux Plugins
Edit `~/.tmux.conf` and add:
```bash
set -g @plugin 'plugin-name'
```
Then press `Prefix + I` to install.

## 🔧 Troubleshooting

### Common Issues

#### Fish Shell Not Set as Default
```bash
# Check available shells
cat /etc/shells

# Add Fish to shells if missing
echo $(which fish) | sudo tee -a /etc/shells

# Set as default shell
chsh -s $(which fish)
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

# Manually add to Fish config
echo "starship init fish | source" >> ~/.config/fish/config.fish
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

## 🔄 Updating

### Update All Configurations
```bash
cd ~/dotfiles
git pull
./install.sh --config-only
```

### Update Fish Plugins
```bash
fisher update
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

## 📂 File Structure

```
~/dotfiles/
├── README.md                    # This file
├── install.sh                   # Main installer script
├── scripts/
│   ├── detect-os.sh            # OS detection utilities
│   ├── install-packages.sh     # Package installation
│   ├── setup-fish.sh           # Fish shell setup
│   ├── setup-tmux.sh           # Tmux setup
│   ├── setup-neovim.sh         # Neovim setup
│   └── setup-starship.sh       # Starship setup
├── configs/
│   ├── fish/
│   │   ├── config.fish          # Main Fish configuration
│   │   ├── conf.d/
│   │   │   ├── abbr.fish        # Abbreviations
│   │   │   ├── paths.fish       # PATH management
│   │   │   └── tmux-mgmt.fish   # Tmux session management
│   │   └── fish_plugins         # Fisher plugins
│   ├── tmux/
│   │   └── .tmux.conf           # Tmux configuration
│   └── starship/
│       └── starship.toml        # Starship configuration
└── backups/                     # Backup directory
```

## 🤝 Contributing

1. Fork this repository
2. Create a feature branch: `git checkout -b feature-name`
3. Make your changes and test them
4. Commit your changes: `git commit -am 'Add feature'`
5. Push to the branch: `git push origin feature-name`
6. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Fish Shell](https://fishshell.com/) - Amazing modern shell
- [Tmux](https://github.com/tmux/tmux) - Terminal multiplexer
- [Neovim](https://neovim.io/) - Hyperextensible Vim-based text editor
- [LazyVim](https://www.lazyvim.org/) - Neovim configuration framework
- [Starship](https://starship.rs/) - Cross-shell prompt
- [Catppuccin](https://github.com/catppuccin/catppuccin) - Beautiful color schemes
- [Fisher](https://github.com/jorgebucaran/fisher) - Fish plugin manager
- [TPM](https://github.com/tmux-plugins/tpm) - Tmux plugin manager

---

**Made with ❤️ for developers who love beautiful, functional terminals**