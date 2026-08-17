vim.g.loaded_netrwPlugin = 1
vim.g.mapleader = ' '
vim.g.maplocalleader = ' '
vim.env.PATH = vim.fn.stdpath('data') .. '/mason/bin:' .. vim.env.PATH
vim.o.clipboard = 'unnamedplus'

-- Over SSH (incl. nested tmux), force OSC 52 so yanks reach the local
-- system clipboard. Built-in auto-detection is gated on $SSH_TTY which
-- tmux often drops; SSH_CONNECTION survives tmux and is set by sshd.
-- Requires `set -g allow-passthrough on` on every tmux layer in the chain.
-- Herdr bridges clipboard writes, not OSC 52 read responses, including
-- when the server is local (hr <host> attaches; panes often have no SSH_*).
local osc52 = require('vim.ui.clipboard.osc52')
local in_herdr = vim.env.HERDR_ENV == '1'
local over_ssh = vim.env.SSH_CONNECTION or vim.env.SSH_CLIENT or vim.env.SSH_TTY
if in_herdr then
    vim.o.clipboard = ''
    local function no_paste()
        return 0
    end
    vim.g.clipboard = {
        name = 'OSC 52',
        copy = {
            ['+'] = osc52.copy('+'),
            ['*'] = osc52.copy('*'),
        },
        paste = {
            ['+'] = no_paste,
            ['*'] = no_paste,
        },
    }
    vim.api.nvim_create_autocmd('TextYankPost', {
        group = vim.api.nvim_create_augroup('HerdrOsc52Copy', { clear = true }),
        callback = function()
            osc52.copy('+')(vim.v.event.regcontents)
        end,
    })
elseif over_ssh then
    vim.g.clipboard = {
        name = 'OSC 52',
        copy = {
            ['+'] = osc52.copy('+'),
            ['*'] = osc52.copy('*'),
        },
        paste = {
            ['+'] = osc52.paste('+'),
            ['*'] = osc52.paste('*'),
        },
    }
end
vim.o.number = true
vim.o.relativenumber = true

-- Search
vim.o.ignorecase = true
vim.o.smartcase = true

-- UI
vim.o.signcolumn = 'yes'
vim.o.cursorline = true
vim.o.colorcolumn = '80'
vim.o.scrolloff = 8
vim.o.sidescrolloff = 8
vim.o.splitbelow = true
vim.o.splitright = true
vim.o.showmode = false
vim.o.termguicolors = true

-- Editing
vim.o.tabstop = 4
vim.o.shiftwidth = 4
vim.o.expandtab = true
vim.o.smartindent = true
vim.o.wrap = false
vim.o.updatetime = 250
vim.o.timeoutlen = 300

-- Persistence
vim.o.undofile = true
vim.o.swapfile = false

vim.diagnostic.config({
	virtual_text = { spacing = 4, prefix = "●" },
	signs = {
		text = {
			[vim.diagnostic.severity.ERROR] = " ",
			[vim.diagnostic.severity.WARN] = " ",
			[vim.diagnostic.severity.HINT] = " ",
			[vim.diagnostic.severity.INFO] = " ",
		},
	},
	underline = true,
	update_in_insert = false,
	severity_sort = true,
	float = { border = "rounded", source = true },
})
