<script setup lang="ts">
/**
 * 空状态
 *
 * 宪法 3.5 节：**空态是引导位，不是留白位。**
 * 故本组件强制要求 title，并把 action 插槽摆在显眼位置——
 * 一个只写「暂无数据」的空态等于浪费了一整屏。
 *
 * 这里也是可爱允许出场的地方之一：空态属于「用户正在等待 / 刚完成」
 * 之外的第三类时刻——什么都没发生。此时没有信息需要被严肃对待，
 * 放一点主视觉不会喧宾夺主（宪法 7.1 节，且每屏只许一处装饰主体）。
 *
 * tone='error' 时刻意收起插画位：出错的时刻不适合可爱（铁律二），
 * 用户此刻要的是知道发生了什么、下一步做什么。
 */

interface Props {
  title: string
  /** 补充说明。可多段，按数组传 */
  description?: string | readonly string[]
  /** error 态收起装饰，只留信息与动作 */
  tone?: 'normal' | 'error'
  /** 纵向留白。嵌在小容器里时用 compact */
  size?: 'compact' | 'normal'
}

const props = withDefaults(defineProps<Props>(), {
  tone: 'normal',
  size: 'normal',
})

const paragraphs = (): readonly string[] => {
  if (!props.description) return []
  return Array.isArray(props.description)
    ? props.description
    : [props.description as string]
}
</script>

<template>
  <div class="es" :class="[`es--${size}`, `es--${tone}`]">
    <!-- 装饰位。第 5 步投放兔耳等纹样，此前留空不占高度 -->
    <div v-if="tone === 'normal' && $slots.visual" class="es__visual">
      <slot name="visual" />
    </div>

    <h3 class="es__title">{{ title }}</h3>

    <div v-if="paragraphs().length" class="es__desc">
      <p v-for="(p, i) in paragraphs()" :key="i">{{ p }}</p>
    </div>

    <!-- 动作区。空态的价值全在这里——
         「社区还没收录这个游戏」后面必须跟着「换个源试试」或「本地导入」 -->
    <div v-if="$slots.action" class="es__action">
      <slot name="action" />
    </div>
  </div>
</template>

<style scoped>
.es {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.es--normal {
  padding: var(--space-7) var(--space-5);
  gap: var(--space-3);
}

.es--compact {
  padding: var(--space-5) var(--space-4);
  gap: var(--space-2);
}

.es__visual {
  /* 装饰不参与信息传达，故不拦鼠标 */
  pointer-events: none;
  margin-bottom: var(--space-2);
  color: var(--color-text-dim);
}

.es__title {
  margin: 0;
  color: var(--color-text);
  /* 空态主文案是 --text-xl 的两个落点之一（另一个是对比表数字） */
  font-size: var(--text-xl);
  font-weight: var(--weight-medium);
  line-height: var(--leading-tight);
}

.es--compact .es__title {
  font-size: var(--text-md);
}

.es__desc {
  max-width: 42ch;
  margin: 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
}

.es__desc p {
  margin: 0 0 var(--space-1);
}

.es__desc p:last-child {
  margin-bottom: 0;
}

.es__action {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-top: var(--space-2);
}

/* error 态：标题用危险色，但不加图标不加插画。
   紧迫感靠信息本身的具体程度攒出来，不靠视觉喊叫。 */
.es--error .es__title {
  color: var(--state-danger);
  font-size: var(--text-md);
}
</style>
