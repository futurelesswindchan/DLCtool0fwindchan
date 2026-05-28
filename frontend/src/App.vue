<template>
  <div class="app-window">
    <!-- 核心双面板布局 -->
    <div class="split-layout">
      <!-- 左侧：中枢控制柱 -->
      <aside class="left-panel">
        <DropZone
          :steam-path="steamPath"
          :is-processing="isProcessing"
          :file-name="currentFileName"
          @file-drop="handleFileDrop"
          @file-select="selectFile"
          @set-steam-path="handleSetSteamPath"
        />

        <!-- 防倒卖与版权信息 -->
        <div class="anti-resale-info flat-block">
          <a href="https://github.com/futurelesswindchan/DLCtool0fwindchan" target="_blank" class="warning-badge" title="访问 GitHub 仓库">
            ⚠️ 免费开源工具，严禁倒卖！
          </a>
          <div class="info-row">
            <span class="author">作者: 没有未来的小风酱</span>
            <button class="theme-toggle-btn" @click="toggleTheme" :title="isDarkTheme ? '切换浅色' : '切换深色'">
              <span class="theme-icon">{{ isDarkTheme ? '☀️' : '🌙' }}</span>
              <span class="theme-text">{{ isDarkTheme ? '亮色' : '暗色' }}</span>
            </button>
          </div>
        </div>
      </aside>

      <!-- 右侧：主舞台 -->
      <main class="right-panel">
        <template v-if="gameData">
          <!-- 顶部信息 -->
          <GameHeader :game-data="gameData" :installed-count="installedCount" />

          <!-- DLC 卡片阵列区 -->
          <div class="dlc-grid-area">
            <div class="dlc-grid">
              <DlcCard
                v-for="(dlc, index) in gameData.dlcs"
                :key="dlc.appID"
                :dlc="dlc"
                :index="index"
                :is-selected="selectedDLCs.includes(dlc.appID)"
                @toggle="toggleDlcSelection"
              />
            </div>
          </div>

          <!-- 底部操作台 -->
          <ActionBar
            :selected-count="selectedDLCs.length"
            :total-count="gameData.dlcs.length"
            :is-processing="isProcessing"
            :progress-percent="progressPercent"
            :notification="notification"
            @select-all="selectAll"
            @select-none="selectNone"
            @clear-all="removeAllDLCs"
            @install-selected="installSelectedDLCs"
          />
        </template>

        <!-- 空闲状态（占位） -->
        <div v-else class="empty-stage">
          <div class="empty-icon">🕹️</div>
          <p>等待接入游戏数据...</p>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import {
  GetSteamPath,
  SetSteamPath,
  SelectDirectory,
  SelectZipFile,
  ProcessZipFile,
  InstallDLCs,
  RemoveAllDLCs,
  ProcessDroppedFile,
} from "../wailsjs/go/main/App";
import { main } from "../wailsjs/go/models";

// 引入拆分的组件
import DropZone from "./components/DropZone.vue";
import GameHeader from "./components/GameHeader.vue";
import DlcCard from "./components/DlcCard.vue";
import ActionBar from "./components/ActionBar.vue";

type GamePackage = main.GamePackage;
type NotificationType = "info" | "success" | "error" | "progress";
interface Notification {
  type: NotificationType;
  message: string;
}

const isDarkTheme = ref(true);
const steamPath = ref("");
const gameData = ref<GamePackage | null>(null);
const selectedDLCs = ref<string[]>([]);
const isProcessing = ref(false);
const progressPercent = ref(0);
const notification = ref<Notification | null>(null);
const currentFileName = ref<string | null>(null);

const installedCount = computed(() => {
  if (!gameData.value) return 0;
  return gameData.value.dlcs.filter((dlc) => dlc.isInstalled).length;
});

const showNotification = (type: NotificationType, message: string) => {
  notification.value = { type, message };
};

const clearNotification = () => {
  notification.value = null;
};

const setProgress = (percent: number, message: string) => {
  progressPercent.value = percent;
  showNotification("progress", message);
};

const toggleTheme = () => {
  isDarkTheme.value = !isDarkTheme.value;
  document.body.classList.toggle("light-theme", !isDarkTheme.value);
};

const toggleDlcSelection = (appID: string) => {
  const idx = selectedDLCs.value.indexOf(appID);
  if (idx > -1) {
    selectedDLCs.value.splice(idx, 1);
  } else {
    selectedDLCs.value.push(appID);
  }
};

const selectAll = () => {
  if (gameData.value) {
    selectedDLCs.value = gameData.value.dlcs.map((d) => d.appID);
  }
};

const selectNone = () => {
  selectedDLCs.value = [];
};

// === Steam 路径处理 ===
const handleSetSteamPath = async () => {
  try {
    const dirPath = await SelectDirectory();
    if (dirPath) {
      isProcessing.value = true;
      setProgress(50, "正在验证 Steam 路径...");
      await SetSteamPath(dirPath);
      steamPath.value = dirPath;
      showNotification("success", "Steam 路径设置成功！");
    }
  } catch (e: any) {
    showNotification("error", e.message || "设置 Steam 路径失败");
  } finally {
    isProcessing.value = false;
    setTimeout(() => {
      if (notification.value?.type === "success") clearNotification();
    }, 3000);
  }
};

// === 文件处理 ===
const selectFile = async () => {
  try {
    clearNotification();
    const filePath = await SelectZipFile();
    if (filePath) {
      // 提取文件名用于展示
      const name = filePath.split(/[\\/]/).pop() || "未知文件.zip";
      currentFileName.value = name;
      await processFile(filePath);
    }
  } catch (e: any) {
    showNotification("error", e.message || "选择文件失败");
  }
};

const handleFileDrop = async (file: File) => {
  if (!file.name.endsWith(".zip")) {
    showNotification("error", "请选择 .zip 格式的压缩包");
    return;
  }

  clearNotification();
  currentFileName.value = file.name;
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
    selectedDLCs.value = result.dlcs
      .filter((d) => !d.isInstalled)
      .map((d) => d.appID);
    setTimeout(clearNotification, 2000);
  } catch (e: any) {
    showNotification("error", e.message || "处理文件失败");
  } finally {
    isProcessing.value = false;
  }
};

const processFile = async (filePath: string) => {
  try {
    isProcessing.value = true;
    setProgress(30, "正在解析压缩包...");
    const result = await ProcessZipFile(filePath);

    setProgress(100, "解析完成！");
    gameData.value = result;
    selectedDLCs.value = result.dlcs
      .filter((d) => !d.isInstalled)
      .map((d) => d.appID);
    setTimeout(clearNotification, 2000);
  } catch (e: any) {
    showNotification("error", e.message || "解析文件失败");
  } finally {
    isProcessing.value = false;
  }
};

// === 安装与清除 ===
const installSelectedDLCs = async () => {
  if (!gameData.value || selectedDLCs.value.length === 0) return;

  isProcessing.value = true;
  clearNotification();
  setProgress(20, "正在关闭 Steam...");

  try {
    setProgress(50, `正在安装 ${selectedDLCs.value.length} 个 DLC...`);
    const result = await InstallDLCs(gameData.value, selectedDLCs.value);
    progressPercent.value = 100;

    if (result.success) {
      gameData.value.dlcs.forEach((dlc) => {
        if (selectedDLCs.value.includes(dlc.appID)) dlc.isInstalled = true;
      });
      showNotification("success", result.message);
    } else {
      showNotification("error", result.message);
    }
  } catch (e: any) {
    showNotification("error", e.message || "安装出错");
  } finally {
    isProcessing.value = false;
    setTimeout(() => {
      if (notification.value?.type === "success") clearNotification();
    }, 3000);
  }
};

const removeAllDLCs = async () => {
  if (!gameData.value) return;

  isProcessing.value = true;
  clearNotification();
  setProgress(20, "正在关闭 Steam...");

  try {
    setProgress(50, `正在清除 ${gameData.value.dlcs.length} 个 DLC...`);
    const result = await RemoveAllDLCs(gameData.value);
    progressPercent.value = 100;

    if (result.success) {
      gameData.value.dlcs.forEach((dlc) => (dlc.isInstalled = false));
      selectedDLCs.value = [];
      showNotification("success", result.message);
    } else {
      showNotification("error", result.message);
    }
  } catch (e: any) {
    showNotification("error", e.message || "清除出错");
  } finally {
    isProcessing.value = false;
    setTimeout(() => {
      if (notification.value?.type === "success") clearNotification();
    }, 3000);
  }
};

// === 生命周期 ===
const onGlobalDragOver = (e: Event) => e.preventDefault();
const onGlobalDrop = (e: Event) => e.preventDefault();

onMounted(async () => {
  window.addEventListener("dragover", onGlobalDragOver);
  window.addEventListener("drop", onGlobalDrop);
  try {
    steamPath.value = await GetSteamPath();
  } catch (e: any) {
    showNotification("error", e.message || "无法找到 Steam 安装路径");
  }
});

onUnmounted(() => {
  window.removeEventListener("dragover", onGlobalDragOver);
  window.removeEventListener("drop", onGlobalDrop);
});
</script>

<style scoped>
.app-window {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background-color: var(--bg-base);
}

/* 核心布局 */
.split-layout {
  flex: 1;
  display: flex;
  overflow: hidden;
  padding: 1.5rem;
  gap: 1.5rem;
}

.left-panel {
  flex: 0 0 320px;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

/* 让 DropZone 占满剩余空间 */
.left-panel > :first-child {
  flex: 1;
  background-color: var(--bg-panel);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-light);
  overflow: hidden;
}

.right-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  background-color: var(--bg-base);
}

/* 防倒卖信息区 */
.anti-resale-info {
  background-color: var(--bg-panel);
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  font-size: 0.8rem;
}

.warning-badge {
  color: var(--accent-warning);
  font-weight: 600;
  text-align: center;
  background-color: rgba(245, 158, 11, 0.1);
  padding: 0.4rem;
  border-radius: var(--radius-sm);
  border: 1px dashed rgba(245, 158, 11, 0.3);
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.author {
  color: var(--text-muted);
}

.theme-toggle-btn {
  background: transparent;
  border: none;
  cursor: pointer;
  font-size: 1rem;
  padding: 0.2rem;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.theme-toggle-btn:hover {
  background-color: var(--bg-card);
}

/* DLC 网格区 */
.dlc-grid-area {
  flex: 1;
  overflow-y: auto;
  padding: 1.5rem;
}

.dlc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1rem;
}

/* 空状态 */
.empty-stage {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  gap: 1rem;
}

.empty-icon {
  font-size: 4rem;
  opacity: 0.5;
}
</style>
