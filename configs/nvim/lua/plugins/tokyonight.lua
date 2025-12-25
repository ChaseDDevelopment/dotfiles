return {
  "folke/tokyonight.nvim",
  name = "tokyonight",
  priority = 1000, -- Ensure it's loaded early
  lazy = false,
  config = function()
    require("tokyonight").setup({
      style = "night", -- The darkest variant: storm, night, moon, day
      transparent = true, -- Enable transparent background
      terminal_colors = false, -- Disable setting terminal colors
      styles = {
        comments = { italic = true },
        keywords = { italic = true },
        functions = {},
        variables = {},
      },
      dim_inactive = false, -- Disable dimming inactive windows
      plugins = {
        cmp = true,
        gitsigns = true,
        nvim_tree = true,
        treesitter = true,
        notify = true,
        mini = true,
      },
      on_colors = function(colors) end,
      on_highlights = function(highlights, colors) end,
    })

    -- Setup must be called before loading
    vim.cmd.colorscheme("tokyonight-night")
  end,
}
