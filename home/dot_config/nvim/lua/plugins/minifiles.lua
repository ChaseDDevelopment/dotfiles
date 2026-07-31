require("mini.files").setup()

-- Floating file editor rooted at the current file (snacks explorer stays on
-- <leader>e for the tree view).
vim.keymap.set("n", "<leader>fm", function()
	local file = vim.api.nvim_buf_get_name(0)
	MiniFiles.open(file ~= "" and file or nil)
end, { desc = "Mini files (current file)" })
