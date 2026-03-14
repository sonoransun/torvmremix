package com.torvm.android.ui.screen

import android.content.Intent
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Share
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.IconToggleButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.torvm.android.vpn.TorVpnService

enum class LogLevel(val label: String) {
    ERROR("Error"),
    WARNING("Warning"),
    INFO("Info"),
    DEBUG("Debug")
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LogScreen(
    onNavigateBack: () -> Unit
) {
    val vpnState by TorVpnService.state.collectAsStateWithLifecycle()
    val errorMessage by TorVpnService.errorMessage.collectAsStateWithLifecycle()
    val bootstrapProgress by TorVpnService.bootstrapProgress.collectAsStateWithLifecycle()
    val logs = remember { mutableStateListOf<String>() }

    var searchQuery by remember { mutableStateOf("") }
    var selectedLevels by remember { mutableStateOf(LogLevel.entries.toSet()) }
    var autoScroll by remember { mutableStateOf(true) }
    val listState = rememberLazyListState()
    val context = LocalContext.current

    LaunchedEffect(vpnState) {
        val timestamp = java.text.SimpleDateFormat("HH:mm:ss", java.util.Locale.US).format(java.util.Date())
        logs.add("[$timestamp] [Info] State: $vpnState")
    }
    LaunchedEffect(errorMessage) {
        errorMessage?.let {
            val timestamp = java.text.SimpleDateFormat("HH:mm:ss", java.util.Locale.US).format(java.util.Date())
            logs.add("[$timestamp] [Error] $it")
        }
    }
    LaunchedEffect(bootstrapProgress) {
        bootstrapProgress?.let {
            val timestamp = java.text.SimpleDateFormat("HH:mm:ss", java.util.Locale.US).format(java.util.Date())
            logs.add("[$timestamp] [Debug] Bootstrap: $it")
        }
    }

    val filteredLogs by remember {
        derivedStateOf {
            logs.filter { log ->
                val matchesSearch = searchQuery.isBlank() ||
                    log.contains(searchQuery, ignoreCase = true)
                val matchesLevel = selectedLevels.any { level ->
                    log.contains("[${level.label}]", ignoreCase = true)
                } || selectedLevels.size == LogLevel.entries.size
                matchesSearch && matchesLevel
            }
        }
    }

    // Auto-scroll to bottom when new logs arrive
    LaunchedEffect(filteredLogs.size) {
        if (autoScroll && filteredLogs.isNotEmpty()) {
            listState.animateScrollToItem(filteredLogs.size - 1)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Logs") },
                navigationIcon = {
                    IconButton(
                        onClick = onNavigateBack,
                        modifier = Modifier.semantics { contentDescription = "Navigate back" }
                    ) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconToggleButton(
                        checked = autoScroll,
                        onCheckedChange = { autoScroll = it },
                        modifier = Modifier.semantics {
                            contentDescription = if (autoScroll) "Disable auto-scroll" else "Enable auto-scroll"
                        }
                    ) {
                        Icon(
                            Icons.Default.KeyboardArrowDown,
                            contentDescription = "Auto-scroll",
                            tint = if (autoScroll) MaterialTheme.colorScheme.primary
                                else MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    IconButton(
                        onClick = {
                            val shareText = filteredLogs.joinToString("\n")
                            val shareIntent = Intent(Intent.ACTION_SEND).apply {
                                type = "text/plain"
                                putExtra(Intent.EXTRA_SUBJECT, "TorVM Logs")
                                putExtra(Intent.EXTRA_TEXT, shareText)
                            }
                            context.startActivity(Intent.createChooser(shareIntent, "Share Logs"))
                        },
                        modifier = Modifier.semantics { contentDescription = "Share logs" }
                    ) {
                        Icon(Icons.Default.Share, contentDescription = "Share logs")
                    }
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
        ) {
            // Search bar
            OutlinedTextField(
                value = searchQuery,
                onValueChange = { searchQuery = it },
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 8.dp, vertical = 4.dp),
                placeholder = { Text("Search logs...") },
                leadingIcon = { Icon(Icons.Default.Search, contentDescription = "Search") },
                singleLine = true
            )

            // Level filter chips
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState())
                    .padding(horizontal = 8.dp, vertical = 4.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                LogLevel.entries.forEach { level ->
                    FilterChip(
                        selected = level in selectedLevels,
                        onClick = {
                            selectedLevels = if (level in selectedLevels) {
                                selectedLevels - level
                            } else {
                                selectedLevels + level
                            }
                        },
                        label = { Text(level.label) },
                        modifier = Modifier.semantics {
                            contentDescription = "${level.label} log filter"
                        }
                    )
                }
            }

            Spacer(modifier = Modifier.height(4.dp))

            // Log list
            LazyColumn(
                state = listState,
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 8.dp)
            ) {
                items(filteredLogs) { log ->
                    val textColor = when {
                        log.contains("[Error]") -> MaterialTheme.colorScheme.error
                        log.contains("[Warning]") -> MaterialTheme.colorScheme.tertiary
                        log.contains("[Debug]") -> MaterialTheme.colorScheme.onSurfaceVariant
                        else -> MaterialTheme.colorScheme.onSurface
                    }
                    Text(
                        text = log,
                        fontFamily = FontFamily.Monospace,
                        fontSize = 12.sp,
                        color = textColor,
                        modifier = Modifier.padding(vertical = 2.dp)
                    )
                }
            }
        }
    }
}
