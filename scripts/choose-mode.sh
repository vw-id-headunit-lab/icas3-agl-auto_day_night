#!/bin/sh
set -eu
daynight_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
daynight_bin=${DAYNIGHT_BIN:-$daynight_dir/daynight}
daynight_config=${DAYNIGHT_CONFIG:-$daynight_dir/config.json}
[ -x "$daynight_bin" ] || { printf '找不到控制程序，请设置 DAYNIGHT_BIN。\n' >&2; exit 1; }
printf '选择日夜主题控制方式：\n1. 原厂 AUTO（跟随车辆日夜信号）\n2. 自定义固定时间\n3. 日出日落\n0. 停止辅助控制（保留原车当前设置）\n请输入编号：'
IFS= read -r daynight_choice
case "$daynight_choice" in
    1) daynight_mode=native-auto ;;
    2) daynight_mode=fixed ;;
    3) daynight_mode=solar ;;
    0) daynight_mode=off ;;
    *) printf '无效选项。\n' >&2; exit 2 ;;
esac
"$daynight_bin" --config "$daynight_config" select "$daynight_mode"
printf '选择已保存。运行中的控制器会重新读取；未运行时需显式 apply/run。\n'
if [ "$daynight_mode" = off ]; then
    printf 'off 不撤销已经设定的原厂 AUTO。恢复安装前设置请在控制器停止后使用 restore。\n'
fi
