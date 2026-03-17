<template>
  <div class="app-container">
    <header class="app-header">
      <div class="header-left">
        <h1>🎮 DLC解锁工具 v1.0</h1>
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
            >✅成功检测到你的 Steam 路径啦！: {{ steamPath }}</span
          >
          <span v-else-if="steamError">❌ {{ steamError }}</span>
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
          <h2>拖拽DLC解锁压缩包到此处哦！</h2>
          <p>支持 .zip 格式压缩包</p>
          <button class="upload-btn" @click="selectFile" :disabled="!steamPath">
            📁 手动选择文件
          </button>
        </div>

        <div v-if="errorMsg" class="error-toast">❌ {{ errorMsg }}</div>
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

        <div v-if="isProcessing" class="progress-section">
          <div class="progress-bar">
            <div
              class="progress-fill"
              :style="{ width: progressPercent + '%' }"
            ></div>
          </div>
          <p class="progress-text">{{ progressMessage }}</p>
        </div>

        <div
          v-if="resultMsg"
          class="result-toast"
          :class="resultSuccess ? 'result-success' : 'result-error'"
        >
          {{ resultSuccess ? "✅" : "❌" }} {{ resultMsg }}
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
import { ref, computed, onMounted } from "vue";
import {
  GetSteamPath,
  SelectZipFile,
  ProcessZipFile,
  InstallDLCs,
  RemoveAllDLCs,
  ProcessDroppedFile,
} from "../wailsjs/go/main/App";

interface DLCInfo {
  appID: string;
  name: string;
  hasKey: boolean;
  decryptionKey: string;
  isInstalled: boolean;
}

interface DepotInfo {
  depotID: string;
  decryptionKey: string;
  manifestID: string;
  fileSize: number;
}

interface GamePackage {
  mainAppID: string;
  gameName: string;
  depots: DepotInfo[];
  dlcs: DLCInfo[];
  luaContent: string;
  manifestFiles: string[];
}

// 状态
const isDarkTheme = ref(false);
const isDragOver = ref(false);
const steamPath = ref("");
const steamError = ref("");
const gameData = ref<GamePackage | null>(null);
const selectedDLCs = ref<string[]>([]);
const errorMsg = ref("");
const isProcessing = ref(false);
const progressPercent = ref(0);
const progressMessage = ref("");
const resultMsg = ref("");
const resultSuccess = ref(false);

// 计算属性
const installedCount = computed(() => {
  if (!gameData.value) return 0;
  return gameData.value.dlcs.filter((dlc) => dlc.isInstalled).length;
});

// 生命周期
onMounted(async () => {
  try {
    steamPath.value = await GetSteamPath();
  } catch (e: any) {
    steamError.value = e.message || "无法找到 Steam 安装路径";
  }
});

// 主题切换
const toggleTheme = () => {
  isDarkTheme.value = !isDarkTheme.value;
  if (isDarkTheme.value) {
    document.body.classList.remove("light-theme");
  } else {
    document.body.classList.add("light-theme");
  }
};

// 文件选择
const selectFile = async () => {
  try {
    errorMsg.value = "";
    const filePath = await SelectZipFile();
    if (filePath) {
      await processFile(filePath);
    }
  } catch (e: any) {
    errorMsg.value = e.message || "选择文件失败";
  }
};

// 拖拽处理
const handleDrop = async (event: DragEvent) => {
  isDragOver.value = false;
  errorMsg.value = "";

  const files = event.dataTransfer?.files;
  if (!files || files.length === 0) return;

  const file = files[0];
  if (!file.name.endsWith(".zip")) {
    errorMsg.value = "请选择 .zip 格式的压缩包";
    return;
  }

  try {
    isProcessing.value = true;
    progressPercent.value = 10;
    progressMessage.value = "正在读取文件...";

    // 读取文件为 ArrayBuffer
    const arrayBuffer = await file.arrayBuffer();
    const uint8Array = new Uint8Array(arrayBuffer);

    progressPercent.value = 30;
    progressMessage.value = "正在处理拖拽文件...";

    // 调用后端函数处理二进制数据
    const result = await ProcessDroppedFile(file.name, Array.from(uint8Array));

    progressPercent.value = 100;
    progressMessage.value = "解析完成！";

    gameData.value = result;

    // 默认选中未安装的 DLC
    selectedDLCs.value = result.dlcs
      .filter((dlc: DLCInfo) => !dlc.isInstalled)
      .map((dlc: DLCInfo) => dlc.appID);

    isProcessing.value = false;
  } catch (e: any) {
    errorMsg.value = e.message || "处理文件失败";
    isProcessing.value = false;
  }
};

// 处理文件
const processFile = async (filePath: string) => {
  try {
    isProcessing.value = true;
    progressPercent.value = 30;
    progressMessage.value = "正在解析压缩包...";

    const result = await ProcessZipFile(filePath);

    progressPercent.value = 100;
    progressMessage.value = "解析完成！";

    gameData.value = result;

    // 默认选中未安装的 DLC
    selectedDLCs.value = result.dlcs
      .filter((dlc: DLCInfo) => !dlc.isInstalled)
      .map((dlc: DLCInfo) => dlc.appID);

    isProcessing.value = false;
  } catch (e: any) {
    isProcessing.value = false;
    errorMsg.value = e.message || "解析文件失败";
  }
};

// 安装选中的 DLC
const installSelectedDLCs = async () => {
  if (!gameData.value || selectedDLCs.value.length === 0) return;

  // 显示免责声明
  const disclaimer = `声明

【本工具用途】
本工具仅供学习、研究和个人使用。严禁用于商业目的。

【使用风险声明】
✓ 本工具修改 Steam 配置文件，可能影响游戏正常运行
✓ 使用本工具安装的 DLC 不受官方支持
✓ 因使用本工具导致的任何问题，开发者不承担责任
✓ 你需要自行承担所有可能的后果

【法律声明】
✓ 本工具不提供任何形式的保证或担保
✓ 使用本工具即表示你已了解上述风险
✓ 你同意在任何情况下不追究开发者的法律责任

【防诈骗提示】
⚠️  此软件完全免费！
⚠️  如果你花钱购买了此工具，说明你被骗了！

【继续操作】
点击"确定"即表示你已阅读并同意上述所有条款。
点击"取消"将放弃本次操作。

═══════════════════════════════════════════════════════════`;

  if (!confirm(disclaimer)) {
    return;
  }

  if (!confirm(disclaimer)) {
    return;
  }

  isProcessing.value = true;
  resultMsg.value = "";
  progressPercent.value = 20;
  progressMessage.value = "正在关闭 Steam...";

  try {
    progressPercent.value = 50;
    progressMessage.value = `正在安装 ${selectedDLCs.value.length} 个 DLC...`;

    const result = await InstallDLCs(gameData.value, selectedDLCs.value);

    progressPercent.value = 100;
    resultSuccess.value = result.success;

    if (result.success) {
      // 更新已安装状态
      gameData.value.dlcs.forEach((dlc) => {
        if (selectedDLCs.value.includes(dlc.appID)) {
          dlc.isInstalled = true;
        }
      });
      resultMsg.value = `✨ 成功安装 ${selectedDLCs.value.length} 个 DLC！\n\n${result.message}\n\n💡 提示：请重启 Steam 以加载新的 DLC。`;
    } else {
      resultMsg.value = `❌ 安装失败\n\n${result.message}`;
    }
  } catch (e: any) {
    resultSuccess.value = false;
    resultMsg.value = `❌ 安装出错\n\n${e.message || "未知错误"}`;
  } finally {
    isProcessing.value = false;
    progressMessage.value = "";
  }
};

// 清除所有 DLC
const removeAllDLCs = async () => {
  if (!gameData.value) return;

  const confirmMsg = `⚠️ 确认清除操作

游戏: ${gameData.value.gameName}
将清除: ${gameData.value.dlcs.length} 个 DLC

此操作将：
✓ 删除 depotcache 中的清单文件
✓ 从 config.vdf 中移除密钥
✓ 从 Steamtools.lua 中移除 addappid

此操作会关闭 Steam 并修改配置文件。
确定要继续吗？`;

  if (!confirm(confirmMsg)) {
    return;
  }

  isProcessing.value = true;
  resultMsg.value = "";
  progressPercent.value = 20;
  progressMessage.value = "正在关闭 Steam...";

  try {
    progressPercent.value = 50;
    progressMessage.value = `正在清除 ${gameData.value.dlcs.length} 个 DLC...`;

    const result = await RemoveAllDLCs(gameData.value);

    progressPercent.value = 100;
    resultSuccess.value = result.success;

    if (result.success) {
      gameData.value.dlcs.forEach((dlc) => {
        dlc.isInstalled = false;
      });
      selectedDLCs.value = [];
      resultMsg.value = `✨ 成功清除所有 DLC！\n\n${result.message}\n\n💡 提示：请重启 Steam 以完成清除。`;
    } else {
      resultMsg.value = `❌ 清除失败\n\n${result.message}`;
    }
  } catch (e: any) {
    resultSuccess.value = false;
    resultMsg.value = `❌ 清除出错\n\n${e.message || "未知错误"}`;
  } finally {
    isProcessing.value = false;
    progressMessage.value = "";
  }
};

// 全选/全不选
const selectAll = () => {
  if (!gameData.value) return;
  selectedDLCs.value = gameData.value.dlcs.map((dlc) => dlc.appID);
};

const selectNone = () => {
  selectedDLCs.value = [];
};

// 重置
const resetSelection = () => {
  gameData.value = null;
  selectedDLCs.value = [];
  errorMsg.value = "";
  resultMsg.value = "";
  isProcessing.value = false;
};
</script>

<style scoped>
.app-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 24px;
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  -webkit-app-region: drag;
}

.app-header button {
  -webkit-app-region: no-drag;
}

.header-left h1 {
  font-size: 20px;
  font-weight: 600;
  margin: 0;
}

.theme-toggle {
  width: 36px;
  height: 36px;
  padding: 0;
  border-radius: 50%;
  font-size: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.app-main {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

/* Steam 状态 */
.steam-status {
  text-align: center;
  padding: 10px 16px;
  border-radius: 8px;
  margin-bottom: 20px;
  font-size: 13px;
}

.status-ok {
  background-color: rgba(76, 175, 80, 0.1);
  color: var(--color-success);
  border: 1px solid rgba(76, 175, 80, 0.3);
}

.status-error {
  background-color: rgba(244, 67, 54, 0.1);
  color: var(--color-error);
  border: 1px solid rgba(244, 67, 54, 0.3);
}

/* 上传区域 */
.upload-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: calc(100% - 60px);
}

.upload-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  max-width: 500px;
  padding: 48px;
  border: 2px dashed var(--border-color);
  border-radius: 12px;
  background-color: var(--bg-secondary);
  text-align: center;
  transition: all 0.3s ease;
  cursor: pointer;
}

.upload-area.drag-over {
  border-color: var(--color-primary);
  background-color: rgba(74, 158, 255, 0.08);
  transform: scale(1.02);
}

.upload-area:hover {
  border-color: var(--color-primary);
}

.upload-icon {
  font-size: 56px;
  margin-bottom: 16px;
}

.upload-area h2 {
  font-size: 18px;
  margin-bottom: 8px;
}

.upload-area p {
  color: var(--text-secondary);
  margin-bottom: 24px;
  font-size: 14px;
}

.upload-btn {
  padding: 10px 24px;
  font-size: 15px;
}

.error-toast {
  margin-top: 16px;
  padding: 10px 20px;
  background-color: rgba(244, 67, 54, 0.1);
  color: var(--color-error);
  border: 1px solid rgba(244, 67, 54, 0.3);
  border-radius: 8px;
  font-size: 14px;
}

/* 游戏信息 */
.game-section {
  max-width: 800px;
  margin: 0 auto;
}

.game-info {
  background-color: var(--bg-secondary);
  padding: 16px 20px;
  border-radius: 8px;
  margin-bottom: 20px;
  border-left: 4px solid var(--color-primary);
}

.game-info-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.game-info-header h2 {
  margin: 0;
  font-size: 20px;
}

.btn-back {
  background-color: var(--text-secondary);
  padding: 6px 14px;
  font-size: 13px;
}

.game-meta {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
}

.meta-item {
  color: var(--text-secondary);
  font-size: 14px;
}

.meta-item code {
  background-color: var(--bg-primary);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: "Courier New", monospace;
  color: var(--color-primary);
}

/* DLC 列表 */
.dlc-list {
  margin-bottom: 20px;
}

.dlc-list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.dlc-list-header h3 {
  font-size: 15px;
  margin: 0;
}

.select-actions {
  display: flex;
  gap: 8px;
}

.btn-small {
  padding: 4px 10px;
  font-size: 12px;
  background-color: var(--border-color);
  color: var(--text-primary);
}

.btn-small:hover {
  background-color: var(--text-secondary);
  transform: none;
  box-shadow: none;
}

.dlc-items {
  background-color: var(--bg-secondary);
  border-radius: 8px;
  padding: 8px;
  max-height: 320px;
  overflow-y: auto;
}

.dlc-item {
  display: flex;
  align-items: center;
  padding: 8px 10px;
  border-radius: 6px;
  transition: background-color 0.15s ease;
}

.dlc-item:hover {
  background-color: var(--bg-primary);
}

.dlc-item input[type="checkbox"] {
  width: 16px;
  height: 16px;
  margin-right: 10px;
  cursor: pointer;
  flex-shrink: 0;
}

.dlc-item label {
  flex: 1;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  min-width: 0;
}

.dlc-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dlc-id {
  color: var(--text-secondary);
  font-size: 12px;
  font-family: "Courier New", monospace;
  flex-shrink: 0;
}

.installed-badge {
  background-color: var(--color-success);
  color: white;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
  flex-shrink: 0;
}

.key-badge {
  background-color: var(--color-primary);
  color: white;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
  flex-shrink: 0;
}

/* 操作按钮 */
.action-buttons {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.action-buttons button {
  flex: 1;
  padding: 12px 16px;
  font-size: 14px;
}

.btn-primary {
  background-color: var(--color-primary);
}

.btn-primary:hover {
  background-color: var(--color-primary-hover);
}

.btn-danger {
  background-color: var(--color-error);
}

.btn-danger:hover {
  background-color: #d32f2f;
}

/* 进度条 */
.progress-section {
  background-color: var(--bg-secondary);
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}

.progress-bar {
  width: 100%;
  height: 6px;
  background-color: var(--border-color);
  border-radius: 3px;
  overflow: hidden;
  margin-bottom: 10px;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(
    90deg,
    var(--color-primary),
    var(--color-success)
  );
  transition: width 0.4s ease;
}

.progress-text {
  text-align: center;
  color: var(--text-secondary);
  font-size: 13px;
  margin: 0;
}

/* 结果提示 */
.result-toast {
  padding: 12px 20px;
  border-radius: 8px;
  font-size: 14px;
  text-align: center;
}

.result-success {
  background-color: rgba(76, 175, 80, 0.1);
  color: var(--color-success);
  border: 1px solid rgba(76, 175, 80, 0.3);
}

.result-error {
  background-color: rgba(244, 67, 54, 0.1);
  color: var(--color-error);
  border: 1px solid rgba(244, 67, 54, 0.3);
}

/* Footer */
.app-footer {
  padding: 12px 24px;
  background-color: var(--bg-secondary);
  border-top: 1px solid var(--border-color);
  text-align: center;
  color: var(--text-secondary);
  font-size: 12px;
}
</style>
