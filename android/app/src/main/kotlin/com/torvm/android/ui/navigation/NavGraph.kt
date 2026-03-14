package com.torvm.android.ui.navigation

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.torvm.android.ui.screen.FirewallScreen
import com.torvm.android.ui.screen.HomeScreen
import com.torvm.android.ui.screen.LogScreen
import com.torvm.android.ui.screen.OnboardingScreen
import com.torvm.android.ui.screen.SettingsScreen

@Composable
fun TorVmNavGraph(
    navController: NavHostController = rememberNavController()
) {
    val context = LocalContext.current
    val prefs = remember { context.getSharedPreferences("torvm_onboarding", Context.MODE_PRIVATE) }
    val onboardingComplete = remember { prefs.getBoolean("completed", false) }
    val startDestination = if (onboardingComplete) "home" else "onboarding"

    NavHost(navController = navController, startDestination = startDestination) {
        composable("onboarding") {
            OnboardingScreen(
                onComplete = {
                    prefs.edit().putBoolean("completed", true).apply()
                    navController.navigate("home") {
                        popUpTo("onboarding") { inclusive = true }
                    }
                }
            )
        }
        composable("home") {
            HomeScreen(
                onNavigateToSettings = { navController.navigate("settings") },
                onNavigateToLogs = { navController.navigate("logs") }
            )
        }
        composable("settings") {
            SettingsScreen(
                onNavigateBack = { navController.popBackStack() },
                onNavigateToFirewall = { navController.navigate("firewall") }
            )
        }
        composable("logs") {
            LogScreen(
                onNavigateBack = { navController.popBackStack() }
            )
        }
        composable("firewall") {
            FirewallScreen(
                onNavigateBack = { navController.popBackStack() }
            )
        }
    }
}
