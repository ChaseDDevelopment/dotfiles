require("nvim-treesitter").setup()

vim.treesitter.language.register("bash", "zsh")
vim.treesitter.language.register("json", "jsonc")
vim.treesitter.language.register("hcl", "terraform")
vim.treesitter.language.register("hcl", "terraform-vars")
vim.treesitter.language.register("yaml", "yaml.ansible")

-- Parser list is the single source of truth in core/pack (shared with the
-- installer's headless bootstrap).
require("nvim-treesitter").install(require("core.pack").TS_LANGS)

vim.api.nvim_create_autocmd("FileType", {
	pattern = {
		"lua",
		"python",
		"typescript",
		"javascript",
		"typescriptreact",
		"javascriptreact",
		"rust",
		"cs",
		"yaml",
		"json",
		"jsonc",
		"toml",
		"xml",
		"html",
		"css",
		"sh",
		"bash",
		"zsh",
		"dockerfile",
		"hcl",
		"markdown",
		"sql",
		"diff",
		"gitcommit",
		"gitignore",
		"vim",
		"help",
		"terraform",
		"terraform-vars",
		"yaml.ansible",
	},
	callback = function()
		vim.treesitter.start()
		vim.bo.indentexpr = "v:lua.require'nvim-treesitter'.indentexpr()"
	end,
})
