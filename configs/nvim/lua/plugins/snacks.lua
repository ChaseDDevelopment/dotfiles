require("snacks").setup({
	picker = { enabled = true },
	toggle = { enabled = true },
})
-- File explorer
vim.keymap.set("n", "<leader>e", function()
	Snacks.picker.explorer({ cwd = vim.fn.getcwd() })
end, { desc = "File explorer" })

-- File picker keymaps
vim.keymap.set("n", "<leader>ff", function()
	Snacks.picker.files()
end, { desc = "Find files" })
vim.keymap.set("n", "<leader>fg", function()
	Snacks.picker.grep()
end, { desc = "Grep" })
vim.keymap.set("n", "<leader>fb", function()
	Snacks.picker.buffers()
end, { desc = "Buffers" })
vim.keymap.set("n", "<leader>fr", function()
	Snacks.picker.recent()
end, { desc = "Recent files" })
vim.keymap.set("n", "<leader>fs", function()
	Snacks.picker.smart()
end, { desc = "Smart find" })

-- LSP navigation through picker
vim.keymap.set("n", "gd", function()
	Snacks.picker.lsp_definitions()
end, { desc = "Go to definition" })
vim.keymap.set("n", "gr", function()
	Snacks.picker.lsp_references()
end, { desc = "References" })
