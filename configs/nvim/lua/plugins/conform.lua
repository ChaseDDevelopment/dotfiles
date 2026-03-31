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
	format_on_save = function(bufnr)
		if vim.g.disable_autoformat or vim.b[bufnr].disable_autoformat then
			return
		end
		return { timeout_ms = 500, lsp_format = "fallback" }
	end,
	formatters = {
		ruff_format = {
			prepend_args = { "--config", "line-length=79" },
		},
	},
})

vim.keymap.set("n", "<leader>cf", function()
	require("conform").format()
end, { desc = "Format file" })

vim.api.nvim_create_user_command("FormatDisable", function(args)
	if args.bang then
		vim.b.disable_autoformat = true
	else
		vim.g.disable_autoformat = true
	end
end, { desc = "Disable format-on-save (! for buffer only)", bang = true })

vim.api.nvim_create_user_command("FormatEnable", function()
	vim.b.disable_autoformat = false
	vim.g.disable_autoformat = false
end, { desc = "Enable format-on-save" })
