<script setup lang="ts">
/**
 * 图标集
 *
 * 为何自绘而非引入图标库（如 Font Awesome）：
 *   1. 体积——需要的图标只有个位数，图标库最少也要拉进一份字体或一个
 *      运行时（100KB 起），而这里全部图形加起来不到 2KB。
 *   2. 图形词汇一致——LogoMark 与 Ornament 已定下栅格 24px、线宽 1.5px、
 *      圆头、无尖角。通用图标库是另一套语言，混在一起能看出「借来的」。
 *   3. 渲染路径已验证——项目踩过 WebView2 不支持 mask-image 的坑
 *      （见 DECISIONS-3 08-08），内联 SVG 是唯一确认可用的方案。
 *
 * 颜色经 `currentColor` 随文字色走，尺寸默认 1em 随字号走，
 * 双主题一份资产，与 LogoMark 同一机制。
 *
 * 新增图标的规矩：viewBox 一律 24×24，只用 stroke 不用 fill，
 * 线宽由外层 svg 统一给出，路径本身不写 stroke-width。
 */

import type { IconName } from "./types";

defineProps<{
  /** 图标名。加新图标要同时在 types.ts 的 IconName 里登记 */
  name: IconName;
}>();
</script>

<template>
  <svg
    class="icon"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.5"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <!-- 搜索：放大镜。镜柄朝右下，与「向外查找」的方向感一致 -->
    <template v-if="name === 'search'">
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="M15.5 15.5 L21 21" />
    </template>

    <!-- 清单包：纸箱。上盖开口用一条横线表达，不画立体透视 -->
    <template v-else-if="name === 'package'">
      <path d="M3 8.5 L12 4 L21 8.5 L21 17 L12 21.5 L3 17 Z" />
      <path d="M3 8.5 L12 13 L21 8.5" />
      <path d="M12 13 L12 21.5" />
    </template>

    <!-- 库：两本并排的书。书脊在左，与侧栏「书脊圆角」语言呼应 -->
    <template v-else-if="name === 'library'">
      <path d="M4 4.5 L4 19.5 C4 20 4.4 20.5 5 20.5 L10 20.5 L10 4.5 Z" />
      <path d="M10 6 L15.5 4.7 C16 4.6 16.5 4.9 16.6 5.4 L19.8 18.4" />
      <path d="M10 20.5 L19 18.3" />
    </template>

    <!-- 总览：柱状图。三根不等高的柱子，读作「统计」 -->
    <template v-else-if="name === 'chart'">
      <path d="M4 20.5 L20 20.5" />
      <path d="M7.5 20.5 L7.5 13" />
      <path d="M12 20.5 L12 7.5" />
      <path d="M16.5 20.5 L16.5 10.5" />
    </template>

    <!-- 环境：插头。两脚朝上，线缆垂下，读作「连接状态」 -->
    <template v-else-if="name === 'plug'">
      <path d="M9 3.5 L9 8" />
      <path d="M15 3.5 L15 8" />
      <path
        d="M6.5 8 L17.5 8 L17.5 11.5 C17.5 14.5 15.2 17 12 17 C8.8 17 6.5 14.5 6.5 11.5 Z"
      />
      <path d="M12 17 L12 20.5" />
    </template>

    <!-- 清单源：地球。一圈两纬线加一条经线，最少笔画表达「在线」 -->
    <template v-else-if="name === 'globe'">
      <circle cx="12" cy="12" r="8.5" />
      <path d="M3.5 12 L20.5 12" />
      <path d="M12 3.5 C15 6.5 15 17.5 12 20.5 C9 17.5 9 6.5 12 3.5 Z" />
    </template>

    <!-- 外观：调色盘。缺口在右侧，圆点表示颜料 -->
    <template v-else-if="name === 'palette'">
      <path
        d="M12 3.5 C7.3 3.5 3.5 7.3 3.5 12 C3.5 16.7 7.3 20.5 12 20.5 C13.4 20.5 14 19.7 14 18.9 C14 17.9 13.2 17.3 13.2 16.4 C13.2 15.5 14 14.9 15 14.9 L17 14.9 C19 14.9 20.5 13.4 20.5 11.4 C20.5 7 16.7 3.5 12 3.5 Z"
      />
      <circle cx="8" cy="10" r="1.1" />
      <circle cx="12" cy="7.5" r="1.1" />
      <circle cx="16" cy="10" r="1.1" />
    </template>

    <!-- 关于：信息。圆圈加 i，点与竖线分离才读得出是字母 -->
    <template v-else-if="name === 'info'">
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 11 L12 16.5" />
      <path d="M12 7.8 L12 8.2" />
    </template>

    <!-- 五角星。折叠态的 Home 标识，顶点朝上、腰身略收 -->
    <template v-else-if="name === 'star'">
      <path
        d="M12 3.6 L14.6 9.3 L20.8 10.1 L16.3 14.4 L17.5 20.5 L12 17.5 L6.5 20.5 L7.7 14.4 L3.2 10.1 L9.4 9.3 Z"
      />
    </template>

    <!-- 筛选：漏斗。刻意不用放大镜——那是「在线搜索」的符号，
         同一图形指两件事会误导。漏斗专表「从已有内容里筛」 -->
    <template v-else-if="name === 'filter'">
      <path d="M3.5 5 L20.5 5 L14 12.5 L14 19.5 L10 17.5 L10 12.5 Z" />
    </template>

    <!-- 设置：齿轮。六齿，圆角避免尖角 -->
    <template v-else-if="name === 'gear'">
      <circle cx="12" cy="12" r="3.2" />
      <path d="M12 2.8 L12 5.4" />
      <path d="M12 18.6 L12 21.2" />
      <path d="M4 7.4 L6.2 8.7" />
      <path d="M17.8 15.3 L20 16.6" />
      <path d="M4 16.6 L6.2 15.3" />
      <path d="M17.8 8.7 L20 7.4" />
    </template>

    <!-- 警示：三角加感叹号。圆角三角，不出尖角 -->
    <template v-else-if="name === 'warn'">
      <path
        d="M10.6 4.2 C11.2 3.2 12.8 3.2 13.4 4.2 L21 17.8 C21.6 18.8 20.9 20 19.7 20 L4.3 20 C3.1 20 2.4 18.8 3 17.8 Z"
      />
      <path d="M12 9 L12 13.8" />
      <path d="M12 16.6 L12 17" />
    </template>
  </svg>
</template>

<style scoped>
.icon {
  display: inline-block;
  /* 跟字号走，调用方可用 width/height 覆盖 */
  width: 1em;
  height: 1em;
  /* 非 flex 上下文下与同行文字中线对齐 */
  vertical-align: middle;
  /* flex 容器中不参与伸缩 */
  flex: 0 0 auto;
}
</style>
