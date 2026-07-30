require("aerial").setup({
	backends = { "treesitter", "lsp" },
	layout = {
		min_width = 30,
		default_direction = "right",
	},
	on_attach = function(bufnr)
		vim.keymap.set("n", "{", "<cmd>AerialPrev<CR>", { buffer = bufnr, desc = "Prev symbol" })
		vim.keymap.set("n", "}", "<cmd>AerialNext<CR>", { buffer = bufnr, desc = "Next symbol" })
	end,
})
