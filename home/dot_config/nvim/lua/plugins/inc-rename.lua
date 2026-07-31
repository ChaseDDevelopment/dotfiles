require("inc_rename").setup({})

-- Live-preview LSP rename, prefilled with the word under the cursor.
vim.keymap.set("n", "<leader>cr", function()
	return ":IncRename " .. vim.fn.expand("<cword>")
end, { expr = true, desc = "Rename symbol" })
