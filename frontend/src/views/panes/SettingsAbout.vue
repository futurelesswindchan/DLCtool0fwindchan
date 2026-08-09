<script setup lang="ts">
/**
 * 设置 · 关于与诊断
 *
 * 版本号取自后端的编译期注入值（GetAppVersion），不在前端硬编码——
 * 两处各存一份必然出现偏差，而排障时版本号错了会把方向带偏。
 *
 * 诊断包放在本 Pane 而非独立的「调试」小节：宪法 3.6 节设想过拆「调试」，
 * 但实际内容只有诊断包导出与两句警示，单独一屏会空得反常；
 * 而它与「版本号 / 日志路径」同属「报障时要用的东西」，聚在一处更合用。
 * 若后续调试项增多再拆。
 */

import { ref, onMounted } from "vue";
import { useToast } from "../../composables/useToast";
import { useConfirm } from "../../composables/useConfirm";
import {
  getLogPath,
  openDataDir,
  exportDiagnostics,
  clearHistory,
  getAppVersion,
  getBuildInfo,
  getReleasePageURL,
  checkUpdate,
  openURL,
  type UpdateInfo,
  type BuildInfo,
} from "../../api";
import { UiButton } from "../../components/ui";

const toast = useToast();
const confirm = useConfirm();

const logPath = ref("");
const version = ref("");

/**
 * 构建身份。封测期间同一版本号会被反复重新构建，仅凭版本号无法确定
 * 用户手里是哪一次的包，故展示带提交哈希的完整标识供其抄进报障消息。
 */
const build = ref<BuildInfo | null>(null);
const releasePage = ref("");
const update = ref<UpdateInfo | null>(null);
const checking = ref(false);
/** 检查更新的失败原因。与 update 互斥，仅用于在按钮下方就地提示。 */
const updateError = ref("");
/** 诊断包导出中，用于禁用按钮避免重复生成 */
const exporting = ref(false);

onMounted(async () => {
  logPath.value = await getLogPath();
  version.value = await getAppVersion();
  build.value = await getBuildInfo();
  releasePage.value = await getReleasePageURL();
});

/**
 * 检查更新。
 *
 * 失败不弹 Toast 而是就地显示：国内访问 api.github.com 经常直接超时，
 * 这是预期内的常态而非操作失败，弹错会让用户误以为工具坏了。就地提示
 * 配合下方常驻的发布页链接，用户自己就能完成后续动作。
 */
async function onCheckUpdate() {
  checking.value = true;
  update.value = null;
  updateError.value = "";

  try {
    update.value = await checkUpdate();
  } catch (e) {
    updateError.value = e instanceof Error ? e.message : String(e);
  } finally {
    checking.value = false;
  }
}

/** 打开外链。后端只放行 http 与 https，此处只需处理失败提示。 */
async function onOpenURL(url: string) {
  try {
    await openURL(url);
  } catch (e) {
    toast.fromError(e, "打开链接失败");
  }
}

async function onOpenDataDir() {
  try {
    await openDataDir();
  } catch (e) {
    toast.fromError(e, "打开数据目录失败");
  }
}

/**
 * 导出脱敏诊断包。
 *
 * 提示文案刻意强调「已移除凭据」而非只说「已导出」：用户需要知道这个包
 * 可以安全外发，否则仍会犹豫，转而去手动翻目录——那正是本功能要避免的
 * 行为。仅在用户确实填过凭据时才提这句（masked 为真），没填过的人看到
 * 「凭据已移除」只会困惑。
 */
async function onExportDiagnostics() {
  exporting.value = true;
  try {
    const r = await exportDiagnostics();
    toast.success(
      r.masked
        ? `已导出 ${r.fileName}（${r.sizeKB} KB），API 凭据已移除，可安全发送`
        : `已导出 ${r.fileName}（${r.sizeKB} KB），可安全发送`,
    );
  } catch (e) {
    toast.fromError(e, "导出诊断包失败");
  } finally {
    exporting.value = false;
  }
}

/**
 * 清空安装记录。
 *
 * 只动账本，不动已部署的文件——这一点必须在确认文案里讲清楚，否则用户
 * 会以为清空记录等于卸载全部游戏。
 */
async function onClearHistory() {
  const ok = await confirm({
    title: "清空安装记录？",
    body: [
      "仅清空本工具的账本，已部署到 Steam 的清单文件不会被删除。",
      "清空后这些游戏会出现在「已安装」页的对账异常区里。",
    ],
    confirmText: "清空记录",
    danger: true,
  });
  if (!ok) return;

  try {
    toast.success(await clearHistory());
  } catch (e) {
    toast.fromError(e, "清空记录失败");
  }
}
</script>

<template>
  <section class="pane">
    <h2 class="set-block__title">关于与诊断</h2>

    <dl class="set-kv">
      <dt>版本</dt>
      <dd>
        <span class="set-kv__btns">
          <code>{{ build?.label || version || "—" }}</code>
          <UiButton size="sm" :loading="checking" @click="onCheckUpdate">
            {{ checking ? "检查中" : "检查更新" }}
          </UiButton>
        </span>

        <p class="set-hint">
          反馈问题时请连括号里的提交哈希一起提供。同一个版本号可能对应
          多次构建，只说版本号无法确定你手里是哪一个包。
        </p>
        <p v-if="build?.dirty" class="set-hint set-hint--warn">
          此包构建时存在未提交的改动，其对应的代码不在仓库中。
          若非你自行编译，请向发布者反馈。
        </p>

        <p v-if="update?.hasUpdate" class="set-hint set-hint--accent">
          有新版本 <strong>{{ update.latestVersion }}</strong>
          <template v-if="update.publishedAt"
            >（{{ update.publishedAt }}）</template
          >
          ——
          <button class="set-linkbtn" @click="onOpenURL(update.releaseURL)">
            前往下载
          </button>
        </p>
        <p v-else-if="update" class="set-hint">
          已是最新版本（远端最新 {{ update.latestVersion }}）。
        </p>
        <p v-if="updateError" class="set-hint">
          暂时查不到更新信息：{{ updateError }}。可
          <button class="set-linkbtn" @click="onOpenURL(releasePage)">
            手动前往发布页
          </button>
          查看。
        </p>
        <p class="set-hint">
          发布顺序是蓝奏云先行、GitHub Release 收尾，所以这里一旦提示新版本，
          安装包必然已经可以下载。
        </p>
      </dd>

      <dt>日志文件</dt>
      <dd>
        <code>{{ logPath || "—" }}</code>
      </dd>

      <dt>报障诊断包</dt>
      <dd>
        <span class="set-kv__btns">
          <UiButton size="sm" :loading="exporting" @click="onExportDiagnostics">
            {{ exporting ? "正在导出" : "导出诊断包" }}
          </UiButton>
        </span>
        <!--
          「已脱敏」必须写在导出入口旁边，不能只写在成功提示里——
          用户是在点击**之前**决定敢不敢发的（宪法 8.3）。
        -->
        <p class="set-hint">
          打包最近的日志与一份<strong>已移除 API 凭据</strong>的配置副本，
          生成后自动打开所在文件夹。反馈问题时请提供这个文件。
        </p>
        <p class="set-hint set-hint--warn">
          请勿直接分享数据目录里的
          <code>config.json</code>，因为它可能含有你自己申请的 API
          凭据，一旦外泄，额度会被他人消耗，账号也可能被封禁。
        </p>
        <p class="set-hint">
          诊断包不含安装记录与清单内容，只有排查问题所需的日志与环境信息。
        </p>
      </dd>

      <dt>数据目录</dt>
      <dd>
        <span class="set-kv__btns">
          <UiButton size="sm" @click="onOpenDataDir"
            >在文件管理器中打开</UiButton
          >
          <UiButton size="sm" variant="danger" @click="onClearHistory">
            清空安装记录
          </UiButton>
        </span>
        <p class="set-hint">
          数据目录内含配置、安装记录、清单留存与日志。删除整个目录即等同彻底
          卸载本工具，不会影响已部署到 Steam 的清单文件。
        </p>
        <p class="set-hint">
          「清空安装记录」只清账本，不删已部署的清单文件。那些游戏仍会留在 Steam
          库中，只是本工具不再管理它们。如果你需要取消 DLC 入库，请去 已安装
          界面进行彻底移除~
        </p>
      </dd>
    </dl>
  </section>
</template>

<style scoped>
.pane {
  max-width: 760px;
}

/* 强调词不另设颜色，只加字重——这几处强调分布在警示与常规说明里，
   若都染色会让整屏出现四五处彩色文字，反而没有重点 */
strong {
  font-weight: var(--weight-semibold);
}
</style>
