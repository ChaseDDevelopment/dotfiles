vim.lsp.config('*', {
	root_markers = { '.git' },
})

vim.lsp.enable({
	'basedpyright',
	'gopls',
	'lua_ls',
	'vtsls',
	'omnisharp',
	'rust_analyzer',
	'dockerls',
	'docker_compose_language_service',
	'bashls',
	'jsonls',
	'yamlls',
	'html',
	'cssls',
	'taplo',
	'marksman',
})
