require("catppuccin").setup({
	flavour = "mocha",
	integrations = {
		blink_cmp = true,
		flash = true,
		gitsigns = true,
		indent_blankline = { enabled = true },
		mason = true,
		noice = true,
		which_key = true,
		treesitter = true,
		treesitter_context = true,
		aerial = true,
		diffview = true,
		harpoon = true,
		render_markdown = true,
		snacks = true,
	},
})
vim.cmd.colorscheme("catppuccin")
