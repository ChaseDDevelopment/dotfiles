return {
	cmd = { 'pyright-langserver', '--stdio' },
	filetypes = { 'python' },
	root_markers = { 'pyproject.toml', 'pyrightconfig.json', 'setup.py', 'setup.cfg', 'requirements.txt', 'Pipfile', '.git' },
}
