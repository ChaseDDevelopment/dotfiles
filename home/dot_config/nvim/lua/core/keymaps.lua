  -- Save file
  vim.keymap.set('n', '<C-s>', '<cmd>w<cr>', { desc = 'Save file' })

  -- Quit all
  vim.keymap.set('n', '<leader>qq', '<cmd>qa<cr>', { desc = 'Quit all' })

  -- Clear search highlight
  vim.keymap.set('n', '<esc>', '<cmd>noh<cr><esc>', { desc = 'Clear search highlight' })

  -- Window navigation: Ctrl-h/j/k/l provided by vim-tmux-navigator
  -- (seamlessly crosses into adjacent tmux panes at split edges).

  -- Resize windows
  vim.keymap.set('n', '<C-Up>', '<cmd>resize +2<cr>', { desc = 'Increase height' })
  vim.keymap.set('n', '<C-Down>', '<cmd>resize -2<cr>', { desc = 'Decrease height' })
  vim.keymap.set('n', '<C-Left>', '<cmd>vertical resize -2<cr>', { desc = 'Decrease width' })
  vim.keymap.set('n', '<C-Right>', '<cmd>vertical resize +2<cr>', { desc = 'Increase width' })

  -- Window splits
  vim.keymap.set('n', '<leader>-', '<C-W>s', { desc = 'Split below' })
  vim.keymap.set('n', '<leader>|', '<C-W>v', { desc = 'Split right' })
  vim.keymap.set('n', '<leader>wd', '<C-W>c', { desc = 'Close window' })

  -- Buffer navigation
  vim.keymap.set('n', '<S-h>', '<cmd>bprevious<cr>', { desc = 'Previous buffer' })
  vim.keymap.set('n', '<S-l>', '<cmd>bnext<cr>', { desc = 'Next buffer' })
  vim.keymap.set('n', '<leader>bd', '<cmd>bdelete<cr>', { desc = 'Delete buffer' })

  -- Move lines (you already have J/K in visual mode)
  vim.keymap.set('n', '<A-j>', '<cmd>m .+1<cr>==', { desc = 'Move line down' })
  vim.keymap.set('n', '<A-k>', '<cmd>m .-2<cr>==', { desc = 'Move line up' })

  -- Better indenting (stays in visual mode)
  vim.keymap.set('v', '<', '<gv')
  vim.keymap.set('v', '>', '>gv')

  -- Diagnostics
  vim.keymap.set('n', '<leader>cd', vim.diagnostic.open_float, { desc = 'Line diagnostics' })
  vim.keymap.set('n', ']d', function() vim.diagnostic.jump({ count = 1 }) end, { desc = 'Next diagnostic' })
  vim.keymap.set('n', '[d', function() vim.diagnostic.jump({ count = -1 }) end, { desc = 'Prev diagnostic' })

  -- Toggle inlay hints
  vim.keymap.set('n', '<leader>uh', function()
    vim.lsp.inlay_hint.enable(not vim.lsp.inlay_hint.is_enabled())
  end, { desc = 'Toggle inlay hints' })

  -- Diagnostics (trouble.nvim)
  vim.keymap.set('n', '<leader>xx', '<cmd>Trouble diagnostics toggle<cr>', { desc = 'Diagnostics' })
  vim.keymap.set('n', '<leader>xX', '<cmd>Trouble diagnostics toggle filter.buf=0<cr>', { desc = 'Buffer diagnostics' })
  vim.keymap.set('n', '<leader>xq', '<cmd>Trouble qflist toggle<cr>', { desc = 'Quickfix list' })
  vim.keymap.set('n', '<leader>xl', '<cmd>Trouble loclist toggle<cr>', { desc = 'Location list' })
  vim.keymap.set('n', '<leader>xt', '<cmd>Trouble todo toggle<cr>', { desc = 'Todo list' })
  vim.keymap.set('n', '[q', '<cmd>cprev<cr>', { desc = 'Prev quickfix' })
  vim.keymap.set('n', ']q', '<cmd>cnext<cr>', { desc = 'Next quickfix' })

  -- New file
  vim.keymap.set('n', '<leader>fn', '<cmd>enew<cr>', { desc = 'New file' })

  -- Plugin management
  vim.keymap.set('n', '<leader>pu', function() vim.pack.update() end, { desc = 'Update plugins' })

  -- Aerial (code outline)
  vim.keymap.set('n', '<leader>o', '<cmd>AerialToggle!<CR>', { desc = 'Code outline' })

  -- Harpoon (quick file nav -- lazy require to avoid startup error)
  vim.keymap.set('n', '<leader>ha', function() require('harpoon'):list():add() end, { desc = 'Add file' })
  vim.keymap.set('n', '<leader>hh', function()
    local harpoon = require('harpoon')
    harpoon.ui:toggle_quick_menu(harpoon:list())
  end, { desc = 'Harpoon menu' })
  vim.keymap.set('n', '<leader>1', function() require('harpoon'):list():select(1) end, { desc = 'Harpoon 1' })
  vim.keymap.set('n', '<leader>2', function() require('harpoon'):list():select(2) end, { desc = 'Harpoon 2' })
  vim.keymap.set('n', '<leader>3', function() require('harpoon'):list():select(3) end, { desc = 'Harpoon 3' })
  vim.keymap.set('n', '<leader>4', function() require('harpoon'):list():select(4) end, { desc = 'Harpoon 4' })
  vim.keymap.set('n', '<leader>5', function() require('harpoon'):list():select(5) end, { desc = 'Harpoon 5' })
  vim.keymap.set('n', '<leader>hp', function() require('harpoon'):list():prev() end, { desc = 'Harpoon prev' })
  vim.keymap.set('n', '<leader>hn', function() require('harpoon'):list():next() end, { desc = 'Harpoon next' })

  -- Git
  vim.keymap.set('n', '<leader>gb', function() require('gitsigns').blame_line({ full = true }) end, { desc = 'Blame line' })

  -- Diffview
  vim.keymap.set('n', '<leader>gd', '<cmd>DiffviewOpen<cr>', { desc = 'Diff view' })
  vim.keymap.set('n', '<leader>gD', '<cmd>DiffviewFileHistory %<cr>', { desc = 'File history' })
  vim.keymap.set('n', '<leader>gq', '<cmd>DiffviewClose<cr>', { desc = 'Close diff view' })

  -- Persistence (sessions)
  vim.keymap.set('n', '<leader>qs', function() require('persistence').load() end, { desc = 'Restore session' })
  vim.keymap.set('n', '<leader>qS', function() require('persistence').select() end, { desc = 'Select session' })
  vim.keymap.set('n', '<leader>qd', function() require('persistence').stop() end, { desc = "Don't save session" })

  -- Toggles
  vim.keymap.set('n', '<leader>uw', function()
    vim.wo.wrap = not vim.wo.wrap
    vim.wo.linebreak = vim.wo.wrap
  end, { desc = 'Toggle word wrap' })
  vim.keymap.set('n', '<leader>uH', '<cmd>Hardtime toggle<cr>', { desc = 'Toggle hardtime' })
  vim.keymap.set('n', '<leader>uP', function() require('precognition').toggle() end, { desc = 'Toggle precognition' })

  -- Claude Code
  vim.keymap.set('n', '<leader>ac', '<cmd>ClaudeCode<cr>', { desc = 'Toggle Claude' })
  vim.keymap.set('n', '<leader>af', '<cmd>ClaudeCodeFocus<cr>', { desc = 'Focus Claude' })
  vim.keymap.set('n', '<leader>ar', '<cmd>ClaudeCode --resume<cr>', { desc = 'Resume Claude' })
  vim.keymap.set('n', '<leader>aC', '<cmd>ClaudeCode --continue<cr>', { desc = 'Continue Claude' })
  vim.keymap.set('v', '<leader>as', '<cmd>ClaudeCodeSend<cr>', { desc = 'Send to Claude' })
  vim.keymap.set('n', '<leader>ab', '<cmd>ClaudeCodeAdd %<cr>', { desc = 'Add buffer to Claude' })
  vim.keymap.set('n', '<leader>aa', '<cmd>ClaudeCodeDiffAccept<cr>', { desc = 'Accept diff' })
  vim.keymap.set('n', '<leader>ad', '<cmd>ClaudeCodeDiffDeny<cr>', { desc = 'Deny diff' })
