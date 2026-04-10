require("mason").setup()

require("mason-lspconfig").setup({
	ensure_installed = {
		"basedpyright", "gopls", "lua_ls", "vtsls", "omnisharp",
		"rust_analyzer", "dockerls", "docker_compose_language_service",
		"bashls", "jsonls", "yamlls", "html", "cssls", "taplo", "marksman",
	},
	automatic_enable = false,
})

require("mason-tool-installer").setup({
	ensure_installed = {
		"ruff",
		"goimports",
		"prettier",
		"stylua",
		"shfmt",
		"shellcheck",
		"markdownlint",
		"csharpier",
		"sqlfluff",
	},
})
