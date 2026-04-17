local function has(bin)
	return vim.fn.executable(bin) == 1
end

local formatters_by_ft = {
	lua = { "stylua" },
	sh = { "shfmt" },
	bash = { "shfmt" },
	zsh = { "shfmt" },
	toml = { "taplo" },
	json = { "prettier" },
	jsonc = { "prettier" },
	yaml = { "prettier" },
	html = { "prettier" },
	css = { "prettier" },
	markdown = { "prettier" },
}
if has("node") then
	formatters_by_ft.typescript = { "prettier" }
	formatters_by_ft.javascript = { "prettier" }
	formatters_by_ft.typescriptreact = { "prettier" }
	formatters_by_ft.javascriptreact = { "prettier" }
end
if has("python3") then
	formatters_by_ft.python = { "ruff_format" }
	formatters_by_ft.sql = { "sqlfluff" }
end
if has("go") then
	formatters_by_ft.go = { "goimports" }
end
if has("cargo") then
	formatters_by_ft.rust = { "rustfmt" }
end
if has("dotnet") then
	formatters_by_ft.cs = { "csharpier" }
end

require("conform").setup({
	formatters_by_ft = formatters_by_ft,
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
