#!/usr/bin/env bash

# Mock progress bar, taken from
# https://stackoverflow.com/questions/238073/how-to-add-a-progress-bar-to-a-shell-script

percentBar ()  {
    local prct totlen=$((8*$2)) lastchar barstring blankstring;
    printf -v prct %.2f "$1"
    ((prct=10#${prct/.}*totlen/10000, prct%8)) &&
        printf -v lastchar '\\U258%X' $(( 16 - prct%8 )) ||
            lastchar=''
    printf -v barstring '%*s' $((prct/8)) ''
    printf -v barstring '%b' "${barstring// /\\U2588}$lastchar"
    printf -v blankstring '%*s' $(((totlen-prct)/8)) ''
    printf -v "$3" '%s%s' "$barstring" "$blankstring"
}

COLUMNS=$(tput cols)
for i in {0..10000..33} 10000;do i=0$i
    printf -v p %0.2f ${i::-2}.${i: -2}
    percentBar $p $((COLUMNS-9)) bar
    printf '\r|%s|%6.2f%%' "$bar" $p
    read -srt .01 _ && break    # console sleep avoiding fork
done
