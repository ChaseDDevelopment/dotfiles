-- Keymaps are automatically loaded on the VeryLazy event
-- Default keymaps that are always set: https://github.com/LazyVim/LazyVim/blob/main/lua/lazyvim/config/keymaps.lua
-- Add any additional keymaps here

-- Visual mode: Move selected lines up or down
vim.keymap.set("v", "J", ":m '>+1<CR>gv=gv", { desc = "Move selected lines down" })
vim.keymap.set("v", "K", ":m '<-2<CR>gv=gv", { desc = "Move selected lines up" })
-- Normal mode: Half-page jump with cursor centered
vim.keymap.set("n", "<C-d>", "<C-d>zz", { desc = "Scroll half-page down and center" })
vim.keymap.set("n", "<C-u>", "<C-u>zz", { desc = "Scroll half-page up and center" })
