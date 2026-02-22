package com.torvm.android.ui.navigation

import androidx.compose.runtime.Composable
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.torvm.android.ui.screen.HomeScreen
import com.torvm.android.ui.screen.LogScreen
import com.torvm.android.ui.screen.SettingsScreen

@Composable
fun TorVmNavGraph(
    navController: NavHostController = rememberNavController()
) {
    NavHost(navController = navController, startDestination = "home") {
        composable("home") {
            HomeScreen(
                onNavigateToSettings = { navController.navigate("settings") },
                onNavigateToLogs = { navController.navigate("logs") }
            )
        }
        composable("settings") {
            SettingsScreen(
                onNavigateBack = { navController.popBackStack() }
            )
        }
        composable("logs") {
            LogScreen(
                onNavigateBack = { navController.popBackStack() }
            )
        }
    }
}
