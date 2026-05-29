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
          <a
            href="#"
            @click.prevent="openGitHub"
            class="github-link-badge"
            title="访问 GitHub 仓库"
          >
            <span class="github-icon">🔗</span>
            还请来Github仓库点个Star喔>w<
          </a>
          <button class="theme-toggle-large-btn" @click="toggleTheme">
            <span class="theme-icon">{{ isDarkTheme ? "☀️" : "🌙" }}</span>
            <span class="theme-text">{{
              isDarkTheme ? "切换至浅色模式" : "切换至深色模式"
            }}</span>
          </button>
          <div class="info-row">
            <span class="warning-text">
              开源免费软件喵，如果是花钱买来的...<br />说明老大你被坏蛋骗了哦QAQ！
            </span>
            <span class="author">Powered By Futurelesswindchan</span>
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
  BrowserOpenURL,
  WindowSetLightTheme,
  WindowSetDarkTheme,
} from "../wailsjs/runtime/runtime";
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
  if (isDarkTheme.value) {
    WindowSetDarkTheme();
  } else {
    WindowSetLightTheme();
  }
};

const openGitHub = () => {
  BrowserOpenURL("https://github.com/futurelesswindchan/DLCtool0fwindchan");
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
  
  // 初始化窗口主题
  if (isDarkTheme.value) {
    WindowSetDarkTheme();
  } else {
    WindowSetLightTheme();
  }

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

.github-link-badge {
  color: var(--accent-primary);
  font-size: 0.75rem;
  font-family: ui-monospace, monospace;
  text-align: center;
  background-color: rgba(59, 130, 246, 0.1);
  padding: 0.5rem;
  border-radius: var(--radius-sm);
  border: 1px dashed rgba(59, 130, 246, 0.3);
  text-decoration: none;
  word-break: break-all;
  transition: all 0.2s;
}

.github-link-badge:hover {
  background-color: rgba(59, 130, 246, 0.2);
  color: var(--accent-hover);
}

.theme-toggle-large-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  width: 100%;
  padding: 0.75rem;
  background-color: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-sm);
  color: var(--text-main);
  font-size: 0.9rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s var(--anim-smooth);
}

.theme-toggle-large-btn:hover {
  background-color: var(--bg-card-hover);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

.theme-toggle-large-btn:active {
  transform: scale(0.98);
}

.info-row {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  gap: 0.4rem;
  padding-top: 0.5rem;
  text-align: center;
}

.warning-text {
  color: var(--accent-warning);
  font-size: 0.75rem;
  font-weight: 600;
  line-height: 1.4;
}

.author {
  color: var(--text-muted);
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
