<script setup lang="ts">
/**
 * 本地导入
 *
 * 从 `SearchView` 底部折叠区整块搬出，成为与在线搜索平级的入口（宪法 3.4）。
 *
 * 搬走而非复制：留两份会立刻产生「改了一处忘了另一处」的隐患，
 * 而这段逻辑涉及部署落盘，是不能有两套行为的地方。
 *
 * 本地导入并非退路——该站网页端额度是 API 的 4~60 倍，对重度用户而言
 * 手动下载再导入反而是更划算的主路径。文案须体现这一点，
 * 不能写成「在线找不到时可以试试」。
 */

import { ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  processZipFile,
  processDroppedFile,
  selectZipFile,
  installDLCs,
  type GamePackage,
} from '../../api'
import { useLibraryStore } from '../../stores/library'
import { useToast } from '../../composables/useToast'
import DropZone from '../../components/DropZone.vue'
import { UiHelpBadge } from '../../components/ui'
import { glossary } from '../../glossary'

const router = useRouter()
const library = useLibraryStore()
const toast = useToast()

const importing = ref(false)

async function onPickFile() {
  const path = await selectZipFile()
  if (!path) return
  await importPackage(() => processZipFile(path))
}

async function onDropFile(file: File) {
  await importPackage(() => processDroppedFile(file))
}

/**
 * 导入本地清单包并直接部署全部 DLC。
 *
 * 本地导入的语义是「用户已经明确知道自己要装什么」，故默认全选后落盘，
 * 随后跳转到游戏页让用户按需取消——若停在此页要求再点一次安装，
 * 与在线路径的「勾选即生效」模型不一致。
 */
async function importPackage(load: () => Promise<GamePackage>) {
  importing.value = true
  try {
    const pkg = await load()
    const allIDs = pkg.dlcs.map((d) => d.appID)
    const msg = await installDLCs(pkg, allIDs)
    toast.success(msg)
    await library.refresh()
    router.push({ name: 'game', params: { appID: pkg.mainAppID } })
  } catch (e) {
    toast.fromError(e, '导入失败')
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <div class="pane">
    <header class="head">
      <h1 class="head__title">
        本地导入
        <UiHelpBadge :content="glossary['import-package'].short" aria-label="了解「清单包」" />
      </h1>
      <p class="head__sub">
        已经从网页端下载好清单包时走这条路。网页端的每日额度远高于 API，
        重度使用时这里比在线搜索更划算。
      </p>
    </header>

    <DropZone :busy="importing" @drop-file="onDropFile" @pick-file="onPickFile" />

    <p class="tips">
      导入后会自动部署包内全部 DLC，随后可在游戏页按需取消。
      <strong>文件名请避免中文与特殊字符</strong>——注入器遇到非 ASCII 文件名
      会直接放弃解析，表现为 Steam 启动时闪退。
    </p>
  </div>
</template>

<style scoped>
.pane {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
}

.head__title {
  margin: 0 0 var(--space-2);
  font-size: var(--text-lg);
  font-weight: var(--weight-semibold);
}

.head__sub {
  margin: 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
}

.tips {
  margin: 0;
  color: var(--color-text-dim);
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
}

.tips strong {
  /* 新文件直接用新令牌名，不经 legacy 别名——别名是给既有调用点的过渡期通道，
     新代码走它只会让 legacy.css 更难删 */
  color: var(--state-warn);
  font-weight: var(--weight-medium);
}
</style>
