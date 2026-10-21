# The demo terminal's only startup file: bash runs with --noprofile and this as its rcfile, in a
# staged HOME, so nothing of the recorder's own shell reaches the video.
PS1='\[\e[38;2;188;183;175m\]~/stack\[\e[0m\] $ '
HISTFILE=/dev/null
export PAGER=cat
# A blank line and a full-width rule before every prompt, so each command's output stands apart.
# `clear` sets __fresh, so a cleared screen starts with the prompt alone.
clear() { command clear; __fresh=1; }
__rule() {
  if [ -z "$__fresh" ]; then
    printf '\n\e[38;2;88;82;77m%s\e[0m\n' "$(printf '─%.0s' $(seq "$(tput cols)"))"
  fi
  __fresh=
}
PROMPT_COMMAND=__rule
__fresh=1
