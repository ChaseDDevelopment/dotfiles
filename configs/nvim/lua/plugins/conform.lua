require("conform").setup({
	formatters_by_ft = {
		python = { "ruff_format" },
		typescript = { "prettier" },
		javascript = { "prettier" },
		typescriptreact = { "prettier" },
		javascriptreact = { "prettier" },
		json = { "prettier" },
		jsonc = { "prettier" },
		yaml = { "prettier" },
		html = { "prettier" },
		css = { "prettier" },
		markdown = { "prettier" },
		lua = { "stylua" },
		rust = { "rustfmt" },
		cs = { "csharpier" },
		sql = { "sqlfluff" },
		sh = { "shfmt" },
		bash = { "shfmt" },
		zsh = { "shfmt" },
		toml = { "taplo" },
	},
	format_on_save = {
		timeout_ms = 500,
		lsp_format = "fallback",
	},
	formatters = {
		ruff_format = {
			prepend_args = { "--config", "line-length=79" },
		},
	},
})

vim.keymap.set("n", "<leader>cf", function()
	require("conform").format()
end, { desc = "Format file" })
