#!/usr/bin/env bash
# Tmux keybinding cheatsheet popup
# Displayed via: prefix + ?
# Dismiss with any keypress (uses less -R)

# Catppuccin Mocha palette
BLUE='\033[38;2;137;180;250m'    # #89b4fa
MAUVE='\033[38;2;203;166;247m'   # #cba6f7
PEACH='\033[38;2;250;179;135m'   # #fab387
TEXT='\033[38;2;205;214;244m'    # #cdd6f4
SUBTEXT='\033[38;2;166;173;200m' # #a6adc8
SURFACE='\033[38;2;69;71;90m'    # #45475a
RESET='\033[0m'

# Column widths
C1=28
C2=28
C3=28

sep() {
    printf "${SURFACE}%0.s─${RESET}" $(seq 1 $((C1 + C2 + C3 + 4)))
    printf '\n'
}

header() {
    printf "${MAUVE}  %-${C1}s${RESET}" "$1"
    printf "${MAUVE}%-${C2}s${RESET}" "$2"
    printf "${MAUVE}%-${C3}s${RESET}" "$3"
    printf '\n'
}

row() {
    printf "  ${PEACH}%-8s${RESET}${TEXT}%-$((C1 - 8))s${RESET}" "$1" "$2"
    printf "${PEACH}%-8s${RESET}${TEXT}%-$((C2 - 8))s${RESET}" "$3" "$4"
    printf "${PEACH}%-8s${RESET}${TEXT}%-$((C3 - 8))s${RESET}" "$5" "$6"
    printf '\n'
}

{
    printf '\n'
    printf "${BLUE}  ╭──────────────────────────────────────────────────────────────────────────────────╮${RESET}\n"
    printf "${BLUE}  │                            Tmux Keybinding Cheatsheet                            │${RESET}\n"
    printf "${BLUE}  ╰──────────────────────────────────────────────────────────────────────────────────╯${RESET}\n"
    printf '\n'
    printf "  ${SUBTEXT}prefix = ${PEACH}C-Space${RESET}          ${SUBTEXT}All bindings below require prefix unless noted${RESET}\n"
    printf '\n'

    sep
    header "Windows" "Pane Splitting" "Pane Navigation"
    sep
    row "c"       "new window"      "|"       "split horiz"    "h"       "pane left"
    row "n"       "next window"     "-"       "split vert"     "j"       "pane down"
    row "p"       "prev window"     ""        ""               "k"       "pane up"
    row "&"       "kill window"     ""        ""               "l"       "pane right"
    row ","       "rename window"   ""        ""               ""        ""
    printf '\n'

    sep
    header "Pane Resize (repeat)" "Sessions" "Vim-Tmux Nav (no pfx)"
    sep
    row "H"       "resize left 5"   "s"       "list sessions"  "C-h"     "pane left"
    row "J"       "resize down 5"   "d"       "detach"         "C-j"     "pane down"
    row "K"       "resize up 5"     "\$"      "rename session" "C-k"     "pane up"
    row "L"       "resize right 5"  ""        ""               "C-l"     "pane right"
    printf '\n'

    sep
    header "Window Nav (no pfx)" "Copy Mode (vi)" "Custom"
    sep
    row "M-H"     "prev window"     "v"       "begin select"   "y"       "capture output"
    row "M-L"     "next window"     "C-v"     "rect toggle"    "r"       "reload config"
    row ""        ""                "y"       "copy + cancel"  ""        ""
    printf '\n'

    sep
    header "Plugins" "" ""
    sep
    row "Tab"     "extrakto"        "p"       "floax pane"     "I"       "install plugins"
    row "U"       "update plugins"  ""        ""               ""        ""
    printf '\n'

    sep
    printf "  ${SUBTEXT}Mouse: ${TEXT}enabled${RESET}    "
    printf "${SUBTEXT}Vi keys: ${TEXT}enabled${RESET}    "
    printf "${SUBTEXT}Base index: ${TEXT}1${RESET}    "
    printf "${SUBTEXT}Scrollback: ${TEXT}50000${RESET}\n"
    printf '\n'
    printf "  ${SURFACE}Press q to close${RESET}\n"
    printf '\n'
} | less -R
