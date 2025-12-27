if command -v nala &>/dev/null; then
  apt() { command nala "$@" }
  sudo() {
    if [ "$1" = "apt" ]; then
      shift
      command sudo nala "$@"
    else
      command sudo "$@"
    fi
  }
fi
