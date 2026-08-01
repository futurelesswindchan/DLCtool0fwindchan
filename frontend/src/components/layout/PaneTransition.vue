<script setup lang="ts">
/**
 * 内容区切换过渡
 *
 * 同页内换 Pane（侧栏点另一个游戏）属「中等动作」，走 --dur-fast
 * 并只做极小位移——位移大了会让用户以为整页换了，而实际侧栏没动。
 *
 * ⚠️ 只过渡 transform 与 opacity（宪法 14 章第 1 条）。
 *
 * ⚠️ 用 transition 而非 animation：animation 不可中断，用户连点两个游戏时
 *    第二次点击会被迫等第一段播完——那正是「点了没反应，反应过来又追不上」
 *    的来源（宪法 5.4 / motion.css）。
 *
 * mode="out-in" 的取舍：默认的同时进出会让两个 Pane 在切换瞬间重叠，
 * 内容区高度不定时观感更乱。代价是总时长翻倍，故单程压在 fast 档。
 *
 * 本组件自带 `ContentPane` 与 `RouterView`，直接作为 Shell 的第二个根节点
 * 即可。把 ContentPane 包在这里而非交给各 Shell，是为了保证三页的滚动、
 * 留白、最大宽度只定义一次——现状 SearchView 与 GameView 各自写了一份
 * `max-width: 860px; margin: 0 auto`，那种「每页自己记得对齐」的做法
 * 迟早对不齐。
 *
 * ⚠️ 过渡必须挂在 ContentPane **内部**。挂在外面会让滚动容器本身参与位移，
 *    切换瞬间滚动条跟着抖一下，而这正是「界面是拼出来的」那种观感来源。
 */

import ContentPane from './ContentPane.vue'

defineProps<{
  /** 透传给 ContentPane：取消最大宽度限制 */
  wide?: boolean
  /** 透传给 ContentPane：去掉内边距 */
  flush?: boolean
}>()
</script>

<template>
  <ContentPane :wide="wide" :flush="flush">
    <RouterView v-slot="{ Component }">
      <Transition name="pane" mode="out-in">
      <!--
        ⚠️ 绝对不要加 :key（宪法 11.4，速查第 26 条）。

        任何随 appID 变化的 key（包括 route.path、route.fullPath）都会强制
        销毁重建组件，使 GameView 里那段实机验证过的 watch(appID, load)
        永远不触发——且不报任何错。

        不加 key 的实际行为：
          - 切到不同组件（SearchPane -> ImportPane）：vnode 类型不同，
            Transition 正常播放。
          - 同组件换 appID：实例复用、无过渡、watch 触发、内容就地更新。
            这正是 5.2 节要的效果——侧栏指示器滑移，右侧就地换内容，
            而不是整块闪一下。
      -->
        <component :is="Component" />
      </Transition>
    </RouterView>
  </ContentPane>
</template>

<style scoped>
.pane-enter-active,
.pane-leave-active {
  transition: opacity var(--dur-fast) var(--ease-standard),
    transform var(--dur-fast) var(--ease-standard);
}

.pane-enter-from {
  opacity: 0;
  /* 4px 而非跨页那 6~8px：同页切换是「换了右半边」，不是「翻页」 */
  transform: translateY(4px);
}

.pane-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
