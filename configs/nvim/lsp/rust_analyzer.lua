return {
	cmd = { 'rust-analyzer' },
	filetypes = { 'rust' },
	root_markers = { 'Cargo.toml', '.git' },
	settings = {
		['rust-analyzer'] = {
			check = {
				command = 'clippy',
			},
			inlayHints = {
				typeHints = { enable = true },
				parameterHints = { enable = true },
			},
		},
	},
}
