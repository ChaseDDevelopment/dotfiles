require("nvim-treesitter").setup()

vim.treesitter.language.register("bash", "zsh")
vim.treesitter.language.register("json", "jsonc")

require("nvim-treesitter").install({
	"lua",
	"python",
	"typescript",
	"javascript",
	"rust",
	"yaml",
	"json",
	"markdown",
	"bash",
	"dockerfile",
	"tsx",
	"c_sharp",
	"toml",
	"xml",
	"html",
	"css",
	"hcl",
	"markdown_inline",
	"git_config",
	"gitcommit",
	"gitignore",
	"diff",
	"sql",
	"vim",
	"vimdoc",
	"luadoc",
	"regex",
})

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
	},
	callback = function()
		vim.treesitter.start()
		vim.bo.indentexpr = "v:lua.require'nvim-treesitter'.indentexpr()"
	end,
})
