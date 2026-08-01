-- use_as_default_explorer defaults to true, which hijacks `nvim <dir>` and
-- fights the snacks-explorer autocmd (a mini.files float flashes, then the
-- explorer replaces it). Keymap-only usage here.
require("mini.files").setup({
	options = { use_as_default_explorer = false },
})

-- Floating file editor rooted at the current file (snacks explorer stays on
-- <leader>e for the tree view).
vim.keymap.set("n", "<leader>fm", function()
	local file = vim.api.nvim_buf_get_name(0)
	MiniFiles.open(file ~= "" and file or nil)
end, { desc = "Mini files (current file)" })
