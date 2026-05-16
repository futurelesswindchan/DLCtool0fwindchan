<template>
  <div class="app-container">
    <header class="app-header">
      <div class="header-left">
        <h1>🎮 DLC入库工具 v1.2</h1>
      </div>
      <div class="header-right">
        <button
          class="theme-toggle"
          @click="toggleTheme"
          :title="isDarkTheme ? '切换到浅色主题' : '切换到深色主题'"
        >
          {{ isDarkTheme ? "☀️" : "🌙" }}
        </button>
      </div>
    </header>

    <main class="app-main">
      <!-- 上传区域 -->
      <div v-if="!gameData" class="upload-section">
        <div
          class="steam-status"
          :class="steamPath ? 'status-ok' : 'status-error'"
        >
          <span v-if="steamPath"
            >✅ 成功检测到你的 Steam 路径啦！: {{ steamPath }}</span
          >
          <span v-else-if="notification && notification.type === 'error'"
            >❌ {{ notification.message }}</span
          >
          <span v-else>⏳ 正在检测 Steam...</span>
        </div>

        <div
          class="upload-area"
          :class="{ 'drag-over': isDragOver }"
          @dragover.prevent="isDragOver = true"
          @dragleave.prevent="isDragOver = false"
          @drop.prevent="handleDrop"
        >
          <div class="upload-icon">📦</div>
          <h2>拖拽DLC解锁包到此处哦！</h2>
          <p>支持 .zip 格式压缩包</p>
          <button class="upload-btn" @click="selectFile" :disabled="!steamPath">
            📁 手动选择文件
          </button>
        </div>

        <!-- 统一通知：上传阶段的错误提示 -->
        <div
          v-if="notification && notification.type === 'error' && steamPath"
          class="error-toast"
        >
          ❌ {{ notification.message }}
        </div>
      </div>

      <!-- 游戏信息和 DLC 列表 -->
      <div v-else class="game-section">
        <div class="game-info">
          <div class="game-info-header">
            <h2>{{ gameData.gameName }}</h2>
            <button class="btn-back" @click="resetSelection">← 返回</button>
          </div>
          <div class="game-meta">
            <span class="meta-item"
              >🆔 AppID: <code>{{ gameData.mainAppID }}</code></span
            >
            <span class="meta-item"
              >📦 DLC: <strong>{{ gameData.dlcs.length }}</strong> 个</span
            >
            <span class="meta-item"
              >✅ 已安装: <strong>{{ installedCount }}</strong> 个</span
            >
          </div>
        </div>

        <div class="dlc-list">
          <div class="dlc-list-header">
            <h3>DLC 列表</h3>
            <div class="select-actions">
              <button class="btn-small" @click="selectAll">全选</button>
              <button class="btn-small" @click="selectNone">全不选</button>
            </div>
          </div>
          <div class="dlc-items">
            <div v-for="dlc in gameData.dlcs" :key="dlc.appID" class="dlc-item">
              <input
                type="checkbox"
                :id="`dlc-${dlc.appID}`"
                v-model="selectedDLCs"
                :value="dlc.appID"
              />
              <label :for="`dlc-${dlc.appID}`">
                <span class="dlc-name">{{ dlc.name }}</span>
                <span class="dlc-id">{{ dlc.appID }}</span>
                <span v-if="dlc.isInstalled" class="installed-badge"
                  >已安装</span
                >
                <span v-if="dlc.hasKey" class="key-badge">有密钥</span>
              </label>
            </div>
          </div>
        </div>

        <div class="action-buttons">
          <button
            class="btn-danger"
            @click="removeAllDLCs"
            :disabled="isProcessing"
          >
            🗑️ 清除所有
          </button>
          <button
            class="btn-primary"
            @click="installSelectedDLCs"
            :disabled="isProcessing || selectedDLCs.length === 0"
          >
            ✨ 安装选中 ({{ selectedDLCs.length }})
          </button>
        </div>

        <!-- 进度条 -->
        <div v-if="isProcessing" class="progress-section">
          <div class="progress-bar">
            <div
              class="progress-fill"
              :style="{ width: progressPercent + '%' }"
            ></div>
          </div>
          <p class="progress-text">
            {{ notification?.type === "progress" ? notification.message : "" }}
          </p>
        </div>

        <!-- 统一通知：操作结果 -->
        <div
          v-if="
            notification && !isProcessing && notification.type !== 'progress'
          "
          class="result-toast"
          :class="{
            'result-success': notification.type === 'success',
            'result-error': notification.type === 'error',
            'result-info': notification.type === 'info',
          }"
        >
          <span v-if="notification.type === 'success'">✅</span>
          <span v-else-if="notification.type === 'error'">❌</span>
          <span v-else>ℹ️</span>
          {{ notification.message }}
        </div>
      </div>
    </main>

    <footer class="app-footer">
      <p>
        copyright © 2026 by 没有未来的小风酱 |
        此软件在qwq.windchan0v0.xyz免费发布！如果你花钱购买了此工具，说明你被骗了QAQ
      </p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import {
  GetSteamPath,
  SelectZipFile,
  ProcessZipFile,
  InstallDLCs,
  RemoveAllDLCs,
  ProcessDroppedFile,
} from "../wailsjs/go/main/App";
import { main } from "../wailsjs/go/models";

// ============================================================
// 类型定义
// ============================================================
// 从 Wails 生成的 models 中复用类型，确保前后端类型完全一致。
type GamePackage = main.GamePackage;
type DLCInfo = main.DLCInfo;
// type DepotInfo = main.DepotInfo;

/**
 * 统一通知消息的类型枚举。
 * 用于替代原先分散的 errorMsg / steamError / resultMsg / progressMessage。
 */
type NotificationType = "info" | "success" | "error" | "progress";

/** 统一通知消息结构体。 */
interface Notification {
  type: NotificationType;
  message: string;
}

// ============================================================
// 响应式状态
// ============================================================

/** 当前是否为暗色主题。默认 true（应用默认暗色）。 */
const isDarkTheme = ref(true);

/** 拖拽悬停状态，用于 UI 高亮反馈。 */
const isDragOver = ref(false);

/** Steam 安装路径（由后端自动识别或手动设置）。 */
const steamPath = ref("");

/** 解析后的游戏数据包，null 表示尚未加载。 */
const gameData = ref<GamePackage | null>(null);

/** 用户选中的 DLC AppID 列表。 */
const selectedDLCs = ref<string[]>([]);

/** 是否正在执行异步操作（解析/安装/卸载）。 */
const isProcessing = ref(false);

/** 进度百分比（0-100），用于进度条展示。 */
const progressPercent = ref(0);

/**
 * 统一通知状态。
 * 替代原先的 errorMsg、steamError、resultMsg、progressMessage 四套状态。
 * 为 null 时表示无通知需要展示。
 */
const notification = ref<Notification | null>(null);

// ============================================================
// 计算属性
// ============================================================

/** 已安装的 DLC 数量，基于后端检测结果统计。 */
const installedCount = computed(() => {
  if (!gameData.value) return 0;
  return gameData.value.dlcs.filter((dlc) => dlc.isInstalled).length;
});

// ============================================================
// 通知辅助函数
// ============================================================

/**
 * 显示通知消息。
 * @param {NotificationType} type - 通知类型
 * @param {string} message - 通知内容
 */
const showNotification = (type: NotificationType, message: string) => {
  notification.value = { type, message };
};

/** 清除当前通知。 */
const clearNotification = () => {
  notification.value = null;
};

/**
 * 设置进度状态（同时更新进度条和通知消息）。
 * @param {number} percent - 进度百分比
 * @param {string} message - 进度描述文本
 */
const setProgress = (percent: number, message: string) => {
  progressPercent.value = percent;
  showNotification("progress", message);
};

// ============================================================
// 生命周期
// ============================================================

/**
 * 全局 dragover 事件处理器引用。
 * 保存引用以便 onUnmounted 时正确移除监听。
 */
const onGlobalDragOver = (e: Event) => {
  e.preventDefault();
};

/**
 * 全局 drop 事件处理器引用。
 * 防止浏览器默认的文件打开行为。
 */
const onGlobalDrop = (e: Event) => {
  e.preventDefault();
};

onMounted(async () => {
  // 注册全局拖拽事件拦截（防止浏览器默认行为）
  window.addEventListener("dragover", onGlobalDragOver);
  window.addEventListener("drop", onGlobalDrop);

  // 自动检测 Steam 路径
  try {
    steamPath.value = await GetSteamPath();
  } catch (e: any) {
    showNotification("error", e.message || "无法找到 Steam 安装路径");
  }
});

/** 组件卸载时移除全局事件监听，防止内存泄漏和重复注册。 */
onUnmounted(() => {
  window.removeEventListener("dragover", onGlobalDragOver);
  window.removeEventListener("drop", onGlobalDrop);
});

// ============================================================
// 主题切换
// ============================================================

/**
 * 切换深色/浅色主题。
 * 暗色为默认状态（无额外 class），浅色通过 body 添加 light-theme class 实现。
 */
const toggleTheme = () => {
  isDarkTheme.value = !isDarkTheme.value;
  document.body.classList.toggle("light-theme", !isDarkTheme.value);
};

// ============================================================
// 文件处理
// ============================================================

/** 通过系统对话框选择 zip 文件并处理。 */
const selectFile = async () => {
  try {
    clearNotification();
    const filePath = await SelectZipFile();
    if (filePath) {
      await processFile(filePath);
    }
  } catch (e: any) {
    showNotification("error", e.message || "选择文件失败");
  }
};

/**
 * 处理拖拽上传的文件。
 * 改进：校验逻辑前置于 isProcessing 状态切换之前，避免 loading 状态闪烁。
 * @param {DragEvent} event - 拖拽事件对象
 */
const handleDrop = async (event: DragEvent) => {
  isDragOver.value = false;
  clearNotification();

  // 校验前置：在进入 processing 状态之前完成所有格式检查
  const files = event.dataTransfer?.files;
  if (!files || files.length === 0) {
    return;
  }

  const file = files[0];
  if (!file.name.endsWith(".zip")) {
    showNotification("error", "请选择 .zip 格式的压缩包");
    return;
  }

  // 校验通过，进入处理状态
  isProcessing.value = true;
  setProgress(10, "正在读取文件...");

  try {
    const arrayBuffer = await file.arrayBuffer();
    setProgress(50, "正在解压文件...");

    const result = await ProcessDroppedFile(
      file.name,
      Array.from(new Uint8Array(arrayBuffer)),
    );

    setProgress(100, "解析完成！");
    gameData.value = result;

    // 默认选中未安装的 DLC
    selectedDLCs.value = result.dlcs
      .filter((dlc: DLCInfo) => !dlc.isInstalled)
      .map((dlc: DLCInfo) => dlc.appID);
  } catch (e: any) {
    showNotification("error", e.message || "处理文件失败");
  } finally {
    isProcessing.value = false;
  }
};

/**
 * 处理已选择的 zip 文件路径（通用流程）。
 * @param {string} filePath - zip 文件完整路径
 */
const processFile = async (filePath: string) => {
  try {
    isProcessing.value = true;
    setProgress(30, "正在解析压缩包...");

    const result = await ProcessZipFile(filePath);

    setProgress(100, "解析完成！");
    gameData.value = result;

    // 默认选中未安装的 DLC
    selectedDLCs.value = result.dlcs
      .filter((dlc: DLCInfo) => !dlc.isInstalled)
      .map((dlc: DLCInfo) => dlc.appID);
  } catch (e: any) {
    showNotification("error", e.message || "解析文件失败");
  } finally {
    isProcessing.value = false;
  }
};

// ============================================================
// DLC 安装与卸载
// ============================================================

/** 安装用户选中的 DLC。执行前展示免责声明确认框。 */
const installSelectedDLCs = async () => {
  if (!gameData.value || selectedDLCs.value.length === 0) return;

  const disclaimer = `声明\n\n【本工具用途】\n本工具仅供学习、研究和个人使用。严禁用于商业目的。\n\n【使用风险声明】\n✓ 本工具修改 Steam 配置文件，可能影响游戏正常运行\n✓ 使用本工具安装的 DLC 不受官方支持\n✓ 因使用本工具导致的任何问题，开发者不承担责任\n✓ 你需要自行承担所有可能的后果\n\n【法律声明】\n✓ 本工具不提供任何形式的保证或担保\n✓ 使用本工具即表示你已了解上述风险\n✓ 你同意在任何情况下不追究开发者的法律责任\n\n【防诈骗提示】\n⚠️ 此软件完全免费！\n⚠️ 如果你花钱购买了此工具，说明你被骗了！\n\n点击"确定"即表示你已阅读并同意上述所有条款。`;

  if (!confirm(disclaimer)) return;

  isProcessing.value = true;
  clearNotification();
  setProgress(20, "正在关闭 Steam...");

  try {
    setProgress(50, `正在安装 ${selectedDLCs.value.length} 个 DLC...`);

    const result = await InstallDLCs(gameData.value, selectedDLCs.value);
    progressPercent.value = 100;

    if (result.success) {
      // 更新本地已安装状态
      gameData.value.dlcs.forEach((dlc) => {
        if (selectedDLCs.value.includes(dlc.appID)) {
          dlc.isInstalled = true;
        }
      });
      showNotification("success", result.message);
    } else {
      showNotification("error", result.message);
    }
  } catch (e: any) {
    showNotification("error", e.message || "安装出错");
  } finally {
    isProcessing.value = false;
  }
};

/** 清除当前游戏的所有已安装 DLC。执行前展示确认框。 */
const removeAllDLCs = async () => {
  if (!gameData.value) return;

  const confirmMsg = `⚠️ 确认清除操作\n\n游戏: ${gameData.value.gameName}\n将清除: ${gameData.value.dlcs.length} 个 DLC\n\n此操作将：\n✓ 删除 depotcache 中的清单文件\n✓ 从 config.vdf 中移除密钥\n✓ 从 Steamtools.lua 中移除 addappid\n\n此操作会关闭 Steam 并修改配置文件。\n确定要继续吗？`;

  if (!confirm(confirmMsg)) return;

  isProcessing.value = true;
  clearNotification();
  setProgress(20, "正在关闭 Steam...");

  try {
    setProgress(50, `正在清除 ${gameData.value.dlcs.length} 个 DLC...`);

    const result = await RemoveAllDLCs(gameData.value);
    progressPercent.value = 100;

    if (result.success) {
      gameData.value.dlcs.forEach((dlc) => {
        dlc.isInstalled = false;
      });
      selectedDLCs.value = [];
      showNotification("success", result.message);
    } else {
      showNotification("error", result.message);
    }
  } catch (e: any) {
    showNotification("error", e.message || "清除出错");
  } finally {
    isProcessing.value = false;
  }
};

// ============================================================
// 选择操作
// ============================================================

/** 全选所有 DLC。 */
const selectAll = () => {
  if (!gameData.value) return;
  selectedDLCs.value = gameData.value.dlcs.map((dlc) => dlc.appID);
};

/** 取消全部选择。 */
const selectNone = () => {
  selectedDLCs.value = [];
};

/** 重置到初始状态（返回上传页面）。 */
const resetSelection = () => {
  gameData.value = null;
  selectedDLCs.value = [];
  clearNotification();
  isProcessing.value = false;
};
</script>
