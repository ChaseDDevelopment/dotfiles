  -- Save file
  vim.keymap.set('n', '<C-s>', '<cmd>w<cr>', { desc = 'Save file' })

  -- Quit all
  vim.keymap.set('n', '<leader>qq', '<cmd>qa<cr>', { desc = 'Quit all' })

  -- Clear search highlight
  vim.keymap.set('n', '<esc>', '<cmd>noh<cr><esc>', { desc = 'Clear search highlight' })

  -- Better window navigation
  vim.keymap.set('n', '<C-h>', '<C-w>h', { desc = 'Go to left window' })
  vim.keymap.set('n', '<C-j>', '<C-w>j', { desc = 'Go to lower window' })
  vim.keymap.set('n', '<C-k>', '<C-w>k', { desc = 'Go to upper window' })
  vim.keymap.set('n', '<C-l>', '<C-w>l', { desc = 'Go to right window' })

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
  vim.keymap.set('n', ']d', vim.diagnostic.goto_next, { desc = 'Next diagnostic' })
  vim.keymap.set('n', '[d', vim.diagnostic.goto_prev, { desc = 'Prev diagnostic' })

  -- Quickfix
  vim.keymap.set('n', '<leader>xl', '<cmd>lopen<cr>', { desc = 'Location list' })
  vim.keymap.set('n', '<leader>xq', '<cmd>copen<cr>', { desc = 'Quickfix list' })
  vim.keymap.set('n', '[q', '<cmd>cprev<cr>', { desc = 'Prev quickfix' })
  vim.keymap.set('n', ']q', '<cmd>cnext<cr>', { desc = 'Next quickfix' })

  -- New file
  vim.keymap.set('n', '<leader>fn', '<cmd>enew<cr>', { desc = 'New file' })
