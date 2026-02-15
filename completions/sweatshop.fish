complete \
  --command sweatshop \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "attach" \
  --description "attach to a worktree session"

complete \
  --command sweatshop \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "status" \
  --description "show status of all repos and worktrees"

complete \
  --command sweatshop \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "merge" \
  --description "merge current worktree into main"

complete \
  --command sweatshop \
  --no-files \
  --keep-order \
  --condition "__fish_seen_subcommand_from attach" \
  --arguments "(sweatshop completions)"
