#!/bin/sh
set -u

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DEFAULT_OUTPUT_ROOT="$SCRIPT_DIR/output"
SOURCE_DIR=${1:-""}

find_latest_live_dir() {
  if [ ! -d "$DEFAULT_OUTPUT_ROOT" ]; then
    return 1
  fi

  old_ifs=$IFS
  IFS='
'
  set -f
  for dir in $(printf '%s\n' "$DEFAULT_OUTPUT_ROOT"/* | sort -r); do
    if [ -d "$dir" ]; then
      for live_dir in "$dir"/*.live; do
        if [ -d "$live_dir" ]; then
          printf '%s\n' "$dir"
          IFS=$old_ifs
          set +f
          return 0
        fi
      done
    fi
  done
  IFS=$old_ifs
  set +f
  return 1
}

if [ -z "$SOURCE_DIR" ]; then
  SOURCE_DIR=$(find_latest_live_dir) || {
    printf '未自动找到可处理目录，请显式传入源目录\n' >&2
    exit 1
  }
  printf '自动使用最新可处理目录：%s\n' "$SOURCE_DIR"
fi
OUTPUT_DIR=${2:-"$SOURCE_DIR/apple-live"}

while [ "$SOURCE_DIR" != "/" ] && [ "${SOURCE_DIR%/}" != "$SOURCE_DIR" ]; do
  SOURCE_DIR=${SOURCE_DIR%/}
done
while [ "$OUTPUT_DIR" != "/" ] && [ "${OUTPUT_DIR%/}" != "$OUTPUT_DIR" ]; do
  OUTPUT_DIR=${OUTPUT_DIR%/}
done

if ! command -v makelive >/dev/null 2>&1; then
  printf 'makelive 未安装或不在 PATH 中\n' >&2
  exit 1
fi

if [ ! -d "$SOURCE_DIR" ]; then
  printf '源目录不存在：%s\n' "$SOURCE_DIR" >&2
  exit 1
fi

case "$OUTPUT_DIR" in
  "$SOURCE_DIR"/*.live|"$SOURCE_DIR"/*.live/*)
    printf '输出目录不能位于任何 .live 中间包内：%s\n' "$OUTPUT_DIR" >&2
    exit 1
    ;;
esac

mkdir -p "$OUTPUT_DIR" || exit 1

processed=0
failed=0
found=0

for dir in "$SOURCE_DIR"/*.live; do
  if [ ! -d "$dir" ]; then
    continue
  fi

  found=1
  name=$(basename "$dir" .live)
  src_photo="$dir/cover.jpg"
  src_video="$dir/motion.mov"
  dst_photo="$OUTPUT_DIR/$name.jpg"
  dst_video="$OUTPUT_DIR/$name.mov"

  if [ ! -f "$src_photo" ] || [ ! -f "$src_video" ]; then
    printf '跳过 %s：缺少 cover.jpg 或 motion.mov\n' "$name" >&2
    failed=$((failed + 1))
    continue
  fi

  if [ "$src_photo" = "$dst_photo" ] || [ "$src_video" = "$dst_video" ]; then
    printf '跳过 %s：目标文件不能与源文件相同\n' "$name" >&2
    failed=$((failed + 1))
    continue
  fi

  rm -f "$dst_photo" "$dst_video"

  if ! cp "$src_photo" "$dst_photo"; then
    printf '复制封面失败：%s -> %s\n' "$src_photo" "$dst_photo" >&2
    failed=$((failed + 1))
    continue
  fi

  if ! cp "$src_video" "$dst_video"; then
    printf '复制视频失败：%s -> %s\n' "$src_video" "$dst_video" >&2
    rm -f "$dst_photo"
    failed=$((failed + 1))
    continue
  fi

  if makelive --manual "$dst_photo" "$dst_video"; then
    printf '已生成：%s\n' "$name"
    processed=$((processed + 1))
  else
    printf 'makelive 处理失败：%s -> %s, %s\n' "$name" "$dst_photo" "$dst_video" >&2
    rm -f "$dst_photo" "$dst_video"
    failed=$((failed + 1))
  fi
done

if [ "$found" -eq 0 ]; then
  printf '未找到任何 .live 目录：%s\n' "$SOURCE_DIR" >&2
  exit 1
fi

printf '输出目录：%s\n' "$OUTPUT_DIR"
printf '成功：%s，失败：%s\n' "$processed" "$failed"

if [ "$failed" -ne 0 ]; then
  exit 1
fi
