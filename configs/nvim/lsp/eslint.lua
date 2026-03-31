return {
	cmd = { 'vscode-eslint-language-server', '--stdio' },
	filetypes = { 'javascript', 'javascriptreact', 'typescript', 'typescriptreact' },
	root_markers = { '.eslintrc', '.eslintrc.js', '.eslintrc.json', '.eslintrc.yaml', 'eslint.config.js', 'eslint.config.mjs', 'eslint.config.ts', '.git' },
	settings = {
		eslint = {
			validate = 'on',
			workingDirectory = { mode = 'location' },
		},
	},
}
